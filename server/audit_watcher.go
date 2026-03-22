package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

type watcherStatus struct {
	Enabled           bool   `json:"enabled"`
	Running           bool   `json:"running"`
	FilePath          string `json:"file_path"`
	Offset            int64  `json:"offset"`
	LastScanAt        int64  `json:"last_scan_at"`
	LastEventAt       int64  `json:"last_event_at"`
	NotificationsSent int64  `json:"notifications_sent"`
	LastError         string `json:"last_error"`
}

type auditWatcher struct {
	plugin *LoginHelpPlugin
	store  *pluginStore
	cfg    *configuration

	stopCh chan struct{}
	doneCh chan struct{}

	statusLock sync.RWMutex
	status     watcherStatus
}

type rawAuditRecord struct {
	Timestamp any    `json:"timestamp"`
	Status    string `json:"status"`
	EventName string `json:"event_name"`
	Actor     struct {
		UserID string `json:"user_id"`
	} `json:"actor"`
	Event struct {
		Parameters     map[string]any `json:"parameters"`
		ResultingState map[string]any `json:"resulting_state"`
	} `json:"event"`
	Meta struct {
		APIPath string `json:"api_path"`
	} `json:"meta"`
}

type loginAuditEvent struct {
	OccurredAt time.Time
	Status     string
	UserID     string
	LoginID    string
	Email      string
	Username   string
	APIPath    string
}

func newAuditWatcher(p *LoginHelpPlugin, cfg *configuration) *auditWatcher {
	return &auditWatcher{
		plugin: p,
		store:  newPluginStore(p.API),
		cfg:    cfg.clone(),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
		status: watcherStatus{
			Enabled:  cfg.Enabled,
			Running:  false,
			FilePath: cfg.AuditLogPath,
		},
	}
}

func (w *auditWatcher) start() {
	go w.loop()
}

func (w *auditWatcher) stop() {
	select {
	case <-w.doneCh:
		return
	default:
	}

	close(w.stopCh)
	<-w.doneCh
}

func (w *auditWatcher) loop() {
	defer close(w.doneCh)

	w.updateStatus(func(status *watcherStatus) {
		status.Running = true
	})

	if err := w.scanOnce(); err != nil {
		w.setError(err)
		w.plugin.API.LogError("Initial audit log scan failed", "error", err.Error())
	}

	ticker := time.NewTicker(time.Duration(w.cfg.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			w.updateStatus(func(status *watcherStatus) {
				status.Running = false
			})
			return
		case <-ticker.C:
			if err := w.scanOnce(); err != nil {
				w.setError(err)
				w.plugin.API.LogError("Audit log scan failed", "error", err.Error())
			}
		}
	}
}

func (w *auditWatcher) snapshot() watcherStatus {
	w.statusLock.RLock()
	defer w.statusLock.RUnlock()
	return w.status
}

func (w *auditWatcher) updateStatus(apply func(*watcherStatus)) {
	w.statusLock.Lock()
	defer w.statusLock.Unlock()
	apply(&w.status)
}

func (w *auditWatcher) setError(err error) {
	if err == nil {
		w.updateStatus(func(status *watcherStatus) {
			status.LastError = ""
		})
		return
	}

	w.updateStatus(func(status *watcherStatus) {
		status.LastError = err.Error()
	})
}

func (w *auditWatcher) scanOnce() error {
	file, err := os.Open(w.cfg.AuditLogPath)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat audit log: %w", err)
	}

	cursor, err := w.store.loadCursor(w.cfg.AuditLogPath)
	if err != nil {
		return err
	}

	var offset int64
	if cursor != nil {
		offset = cursor.Offset
	}

	if cursor == nil && w.cfg.StartFromEnd {
		offset = info.Size()
		if err := w.store.saveCursor(&watcherCursor{
			Path:      w.cfg.AuditLogPath,
			Offset:    offset,
			UpdatedAt: time.Now().UTC().Unix(),
		}); err != nil {
			return err
		}

		w.updateStatus(func(status *watcherStatus) {
			status.Offset = offset
			status.LastScanAt = time.Now().UTC().Unix()
			status.LastError = ""
		})

		return nil
	}

	if offset > info.Size() {
		offset = 0
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("seek audit log: %w", err)
	}

	reader := bufio.NewReader(file)
	currentOffset := offset
	for {
		select {
		case <-w.stopCh:
			return nil
		default:
		}

		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			currentOffset += int64(len(line))
			if err := w.handleLine(bytes.TrimSpace(line)); err != nil {
				w.plugin.API.LogWarn("Skipping unreadable audit log line", "error", err.Error())
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}

			return fmt.Errorf("read audit log: %w", readErr)
		}
	}

	nowUnix := time.Now().UTC().Unix()
	if err := w.store.saveCursor(&watcherCursor{
		Path:      w.cfg.AuditLogPath,
		Offset:    currentOffset,
		UpdatedAt: nowUnix,
	}); err != nil {
		return err
	}

	w.updateStatus(func(status *watcherStatus) {
		status.Offset = currentOffset
		status.LastScanAt = nowUnix
		status.LastError = ""
	})

	return nil
}

func (w *auditWatcher) handleLine(line []byte) error {
	if len(line) == 0 {
		return nil
	}

	event, err := parseLoginAuditLine(line)
	if err != nil || event == nil {
		return err
	}

	w.updateStatus(func(status *watcherStatus) {
		status.LastEventAt = event.OccurredAt.UTC().Unix()
	})

	user, err := w.resolveUser(event)
	if err != nil {
		return err
	}

	if user == nil {
		return nil
	}

	if w.cfg.OnlyLocalAccounts && !isLocalMattermostAccount(user) {
		return nil
	}

	if strings.EqualFold(event.Status, "success") {
		if w.cfg.ResetOnSuccess {
			return w.store.deleteFailureState(user.Id)
		}
		return nil
	}

	if !strings.EqualFold(event.Status, "fail") {
		return nil
	}

	return w.recordFailure(user, event)
}

func (w *auditWatcher) resolveUser(event *loginAuditEvent) (*model.User, error) {
	if event.UserID != "" {
		user, appErr := w.plugin.API.GetUser(event.UserID)
		if appErr == nil {
			return user, nil
		}
	}

	if event.Email != "" {
		user, appErr := w.plugin.API.GetUserByEmail(event.Email)
		if appErr == nil {
			return user, nil
		}
	}

	if event.Username != "" {
		user, appErr := w.plugin.API.GetUserByUsername(event.Username)
		if appErr == nil {
			return user, nil
		}
	}

	if event.LoginID != "" {
		if strings.Contains(event.LoginID, "@") {
			user, appErr := w.plugin.API.GetUserByEmail(event.LoginID)
			if appErr == nil {
				return user, nil
			}
		} else {
			user, appErr := w.plugin.API.GetUserByUsername(event.LoginID)
			if appErr == nil {
				return user, nil
			}
		}
	}

	return nil, nil
}

func (w *auditWatcher) recordFailure(user *model.User, event *loginAuditEvent) error {
	state, err := w.store.loadFailureState(user.Id)
	if err != nil {
		return err
	}

	eventUnix := event.OccurredAt.UTC().Unix()
	cutoff := eventUnix - int64(w.cfg.WindowMinutes*60)
	state.Attempts = pruneAttempts(state.Attempts, cutoff)
	state.Attempts = append(state.Attempts, eventUnix)

	attemptCount := len(state.Attempts)
	if attemptCount >= w.cfg.FailureThreshold && canSendAgain(state.LastNotificationAt, eventUnix, int64(w.cfg.CooldownMinutes*60)) {
		if err := w.plugin.sendHelpEmail(user, w.cfg, attemptCount, event.OccurredAt, false); err != nil {
			return err
		}

		state.LastNotificationAt = eventUnix
		w.updateStatus(func(status *watcherStatus) {
			status.NotificationsSent++
		})

		w.plugin.API.LogInfo(
			"Sent login help email",
			"user_id", user.Id,
			"email", user.Email,
			"attempt_count", attemptCount,
			"threshold", w.cfg.FailureThreshold,
			"api_path", event.APIPath,
		)
	}

	return w.store.saveFailureState(state)
}

func parseLoginAuditLine(line []byte) (*loginAuditEvent, error) {
	var record rawAuditRecord
	if err := json.Unmarshal(line, &record); err != nil {
		return nil, fmt.Errorf("decode audit record: %w", err)
	}

	if !strings.EqualFold(record.EventName, "login") {
		return nil, nil
	}

	occurredAt, err := parseAuditTimestamp(record.Timestamp)
	if err != nil {
		occurredAt = time.Now().UTC()
	}

	parameters := any(record.Event.Parameters)
	resultingState := any(record.Event.ResultingState)

	event := &loginAuditEvent{
		OccurredAt: occurredAt,
		Status:     strings.ToLower(strings.TrimSpace(record.Status)),
		UserID: firstNonEmpty(
			record.Actor.UserID,
			findStringRecursive(parameters, "user_id"),
			findStringRecursive(resultingState, "user_id"),
		),
		LoginID: firstNonEmpty(
			findStringRecursive(parameters, "login_id", "loginid"),
			findStringRecursive(resultingState, "login_id", "loginid"),
		),
		Email: firstNonEmpty(
			findStringRecursive(parameters, "email"),
			findStringRecursive(resultingState, "email"),
		),
		Username: firstNonEmpty(
			findStringRecursive(parameters, "username"),
			findStringRecursive(resultingState, "username"),
		),
		APIPath: record.Meta.APIPath,
	}

	return event, nil
}

func parseAuditTimestamp(raw any) (time.Time, error) {
	switch value := raw.(type) {
	case string:
		value = strings.TrimSpace(value)
		layouts := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05.000 Z07:00",
			"2006-01-02 15:04:05 Z07:00",
			"2006-01-02 15:04:05.000 -07:00",
			"2006-01-02 15:04:05 -07:00",
		}

		for _, layout := range layouts {
			if parsed, err := time.Parse(layout, value); err == nil {
				return parsed.UTC(), nil
			}
		}

		return time.Time{}, fmt.Errorf("unsupported timestamp format: %q", value)
	case float64:
		return unixFromUnknownPrecision(int64(value)), nil
	case int64:
		return unixFromUnknownPrecision(value), nil
	case json.Number:
		number, err := value.Int64()
		if err != nil {
			return time.Time{}, fmt.Errorf("parse numeric timestamp: %w", err)
		}
		return unixFromUnknownPrecision(number), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported timestamp type %T", raw)
	}
}

func unixFromUnknownPrecision(value int64) time.Time {
	switch {
	case value > 1_000_000_000_000_000:
		return time.UnixMicro(value).UTC()
	case value > 1_000_000_000_000:
		return time.UnixMilli(value).UTC()
	default:
		return time.Unix(value, 0).UTC()
	}
}

func findStringRecursive(value any, keys ...string) string {
	if value == nil {
		return ""
	}

	lookup := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		lookup[strings.ToLower(key)] = struct{}{}
	}

	return findStringRecursiveWithLookup(value, lookup)
}

func findStringRecursiveWithLookup(value any, lookup map[string]struct{}) string {
	switch typed := value.(type) {
	case map[string]any:
		for key, inner := range typed {
			if _, ok := lookup[strings.ToLower(key)]; ok {
				if asString := stringify(inner); asString != "" {
					return asString
				}
			}
		}

		for _, inner := range typed {
			if found := findStringRecursiveWithLookup(inner, lookup); found != "" {
				return found
			}
		}
	case []any:
		for _, inner := range typed {
			if found := findStringRecursiveWithLookup(inner, lookup); found != "" {
				return found
			}
		}
	}

	return ""
}

func stringify(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

func isLocalMattermostAccount(user *model.User) bool {
	if user == nil {
		return false
	}

	switch strings.ToLower(strings.TrimSpace(user.AuthService)) {
	case "", "email":
		return true
	default:
		return false
	}
}
