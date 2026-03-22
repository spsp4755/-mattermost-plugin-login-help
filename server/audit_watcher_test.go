package main

import (
	"testing"
	"time"
)

func TestParseLoginAuditLine(t *testing.T) {
	line := []byte(`{
		"timestamp": "2026-03-22T12:34:56Z",
		"event_name": "login",
		"status": "fail",
		"actor": {"user_id": "user-123"},
		"event": {
			"parameters": {"login_id": "alice@example.com"},
			"resulting_state": {}
		},
		"meta": {"api_path": "/api/v4/users/login"}
	}`)

	event, err := parseLoginAuditLine(line)
	if err != nil {
		t.Fatalf("expected no parse error, got %v", err)
	}

	if event == nil {
		t.Fatal("expected a login event")
	}

	if event.UserID != "user-123" {
		t.Fatalf("expected user id to be populated, got %q", event.UserID)
	}

	if event.LoginID != "alice@example.com" {
		t.Fatalf("expected login id to be populated, got %q", event.LoginID)
	}

	if event.APIPath != "/api/v4/users/login" {
		t.Fatalf("expected api path to be populated, got %q", event.APIPath)
	}
}

func TestPruneAttempts(t *testing.T) {
	attempts := []int64{10, 20, 30, 40}
	got := pruneAttempts(attempts, 25)

	if len(got) != 2 || got[0] != 30 || got[1] != 40 {
		t.Fatalf("unexpected pruned attempts: %#v", got)
	}
}

func TestCanSendAgain(t *testing.T) {
	now := time.Now().UTC().Unix()

	if !canSendAgain(0, now, 3600) {
		t.Fatal("expected empty notification history to allow send")
	}

	if canSendAgain(now-60, now, 3600) {
		t.Fatal("expected cooldown to block send")
	}

	if !canSendAgain(now-7200, now, 3600) {
		t.Fatal("expected expired cooldown to allow send")
	}
}
