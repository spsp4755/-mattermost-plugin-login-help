package main

import (
	"fmt"
	"strings"
)

const (
	defaultFailureThreshold = 3
	defaultWindowMinutes    = 15
	defaultCooldownMinutes  = 60
	defaultPollInterval     = 5
	defaultEmailSubject     = "[Mattermost] Password reset guide"
)

type configuration struct {
	Enabled             bool   `json:"Enabled"`
	AuditLogPath        string `json:"AuditLogPath"`
	FailureThreshold    int    `json:"FailureThreshold"`
	WindowMinutes       int    `json:"WindowMinutes"`
	CooldownMinutes     int    `json:"CooldownMinutes"`
	ConfluenceURL       string `json:"ConfluenceURL"`
	EmailSubject        string `json:"EmailSubject"`
	PollIntervalSeconds int    `json:"PollIntervalSeconds"`
	StartFromEnd        bool   `json:"StartFromEnd"`
	ResetOnSuccess      bool   `json:"ResetOnSuccess"`
	OnlyLocalAccounts   bool   `json:"OnlyLocalAccounts"`
}

func (c *configuration) setDefaults() {
	if c.FailureThreshold <= 0 {
		c.FailureThreshold = defaultFailureThreshold
	}

	if c.WindowMinutes <= 0 {
		c.WindowMinutes = defaultWindowMinutes
	}

	if c.CooldownMinutes <= 0 {
		c.CooldownMinutes = defaultCooldownMinutes
	}

	if c.PollIntervalSeconds <= 0 {
		c.PollIntervalSeconds = defaultPollInterval
	}

	if strings.TrimSpace(c.EmailSubject) == "" {
		c.EmailSubject = defaultEmailSubject
	}
}

func (c *configuration) validate() error {
	if !c.Enabled {
		return nil
	}

	if strings.TrimSpace(c.AuditLogPath) == "" {
		return fmt.Errorf("AuditLogPath is required")
	}

	if strings.TrimSpace(c.ConfluenceURL) == "" {
		return fmt.Errorf("ConfluenceURL is required")
	}

	if c.FailureThreshold < 1 {
		return fmt.Errorf("FailureThreshold must be at least 1")
	}

	if c.WindowMinutes < 1 {
		return fmt.Errorf("WindowMinutes must be at least 1")
	}

	if c.CooldownMinutes < 0 {
		return fmt.Errorf("CooldownMinutes cannot be negative")
	}

	if c.PollIntervalSeconds < 1 {
		return fmt.Errorf("PollIntervalSeconds must be at least 1")
	}

	return nil
}

func (c *configuration) clone() *configuration {
	if c == nil {
		cfg := configuration{}
		cfg.setDefaults()
		return &cfg
	}

	copy := *c
	return &copy
}
