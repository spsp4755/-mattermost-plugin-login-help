package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	cursorKeyPrefix  = "cursor_"
	failureKeyPrefix = "failure_"
)

type kvStoreAPI interface {
	KVGet(key string) ([]byte, *model.AppError)
	KVSet(key string, value []byte) *model.AppError
	KVDelete(key string) *model.AppError
}

type pluginStore struct {
	api kvStoreAPI
}

type watcherCursor struct {
	Path      string `json:"path"`
	Offset    int64  `json:"offset"`
	UpdatedAt int64  `json:"updated_at"`
}

type failureState struct {
	UserID             string  `json:"user_id"`
	Attempts           []int64 `json:"attempts"`
	LastNotificationAt int64   `json:"last_notification_at"`
}

func newPluginStore(api kvStoreAPI) *pluginStore {
	return &pluginStore{api: api}
}

func (s *pluginStore) loadCursor(path string) (*watcherCursor, error) {
	raw, appErr := s.api.KVGet(cursorKey(path))
	if appErr != nil {
		return nil, fmt.Errorf("load cursor: %s", appErr.Error())
	}

	if len(raw) == 0 {
		return nil, nil
	}

	var cursor watcherCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return nil, fmt.Errorf("decode cursor: %w", err)
	}

	return &cursor, nil
}

func (s *pluginStore) saveCursor(cursor *watcherCursor) error {
	payload, err := json.Marshal(cursor)
	if err != nil {
		return fmt.Errorf("encode cursor: %w", err)
	}

	if appErr := s.api.KVSet(cursorKey(cursor.Path), payload); appErr != nil {
		return fmt.Errorf("save cursor: %s", appErr.Error())
	}

	return nil
}

func (s *pluginStore) loadFailureState(userID string) (*failureState, error) {
	raw, appErr := s.api.KVGet(failureKey(userID))
	if appErr != nil {
		return nil, fmt.Errorf("load failure state: %s", appErr.Error())
	}

	if len(raw) == 0 {
		return &failureState{
			UserID:   userID,
			Attempts: []int64{},
		}, nil
	}

	var state failureState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("decode failure state: %w", err)
	}

	if state.Attempts == nil {
		state.Attempts = []int64{}
	}

	return &state, nil
}

func (s *pluginStore) saveFailureState(state *failureState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode failure state: %w", err)
	}

	if appErr := s.api.KVSet(failureKey(state.UserID), payload); appErr != nil {
		return fmt.Errorf("save failure state: %s", appErr.Error())
	}

	return nil
}

func (s *pluginStore) deleteFailureState(userID string) error {
	if appErr := s.api.KVDelete(failureKey(userID)); appErr != nil {
		return fmt.Errorf("delete failure state: %s", appErr.Error())
	}

	return nil
}

func cursorKey(path string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(path))))
	return cursorKeyPrefix + hex.EncodeToString(sum[:8])
}

func failureKey(userID string) string {
	return failureKeyPrefix + strings.TrimSpace(userID)
}

func pruneAttempts(attempts []int64, cutoff int64) []int64 {
	if len(attempts) == 0 {
		return []int64{}
	}

	pruned := make([]int64, 0, len(attempts))
	for _, attempt := range attempts {
		if attempt >= cutoff {
			pruned = append(pruned, attempt)
		}
	}

	return pruned
}

func canSendAgain(lastNotificationAt, nowUnix, cooldownSeconds int64) bool {
	if lastNotificationAt == 0 || cooldownSeconds <= 0 {
		return true
	}

	return nowUnix-lastNotificationAt >= cooldownSeconds
}
