package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

type statusResponse struct {
	Configuration   *configuration `json:"configuration"`
	ValidationError string         `json:"validation_error"`
	Watcher         watcherStatus  `json:"watcher"`
	ServerTimeUTC   string         `json:"server_time_utc"`
}

func (p *LoginHelpPlugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/status":
		p.handleStatus(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/test-mail":
		p.handleTestMail(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (p *LoginHelpPlugin) handleStatus(w http.ResponseWriter, r *http.Request) {
	_, err := p.requireSystemAdmin(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}

	writeJSON(w, http.StatusOK, statusResponse{
		Configuration:   p.getConfiguration(),
		ValidationError: p.getValidationError(),
		Watcher:         p.getWatcherStatus(),
		ServerTimeUTC:   time.Now().UTC().Format(time.RFC3339),
	})
}

func (p *LoginHelpPlugin) handleTestMail(w http.ResponseWriter, r *http.Request) {
	user, err := p.requireSystemAdmin(r)
	if err != nil {
		writeError(w, err, http.StatusUnauthorized)
		return
	}

	cfg := p.getConfiguration()
	if strings.TrimSpace(cfg.ConfluenceURL) == "" {
		writeError(w, fmt.Errorf("ConfluenceURL must be configured before sending a test mail"), http.StatusBadRequest)
		return
	}

	if err := p.sendHelpEmail(user, cfg, cfg.FailureThreshold, time.Now().UTC(), true); err != nil {
		writeError(w, err, http.StatusBadGateway)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "test email sent",
		"user_id": user.Id,
		"email":   user.Email,
	})
}

func (p *LoginHelpPlugin) requireSystemAdmin(r *http.Request) (*model.User, error) {
	userID := strings.TrimSpace(r.Header.Get("Mattermost-User-Id"))
	if userID == "" {
		return nil, fmt.Errorf("authentication required")
	}

	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return nil, fmt.Errorf("load user: %s", appErr.Error())
	}

	if !roleContains(user.Roles, "system_admin") {
		return nil, fmt.Errorf("system administrator access is required")
	}

	return user, nil
}

func roleContains(roles, target string) bool {
	for _, role := range strings.Fields(roles) {
		if role == target {
			return true
		}
	}

	return false
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error, status int) {
	writeJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}
