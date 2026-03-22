package main

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

var helpEmailTemplate = template.Must(template.New("help_email").Parse(`
<html>
  <body style="font-family: Arial, sans-serif; line-height: 1.5; color: #1f2933;">
    <p>Repeated Mattermost login failures were detected for your account.</p>
    <p>Mattermost 로그인 실패가 여러 차례 감지되었습니다.</p>
    <p>Please use the internal password reset guide below.</p>
    <p><a href="{{.ConfluenceURL}}">{{.ConfluenceURL}}</a></p>
    <p>Matched failures: {{.AttemptCount}} within {{.WindowMinutes}} minutes.</p>
    <p>Detected at: {{.TriggeredAt}}</p>
    {{if .IsTest}}
    <p>This is a test email sent by a Mattermost system administrator.</p>
    {{end}}
  </body>
</html>
`))

type helpEmailData struct {
	ConfluenceURL string
	AttemptCount  int
	WindowMinutes int
	TriggeredAt   string
	IsTest        bool
}

func (p *LoginHelpPlugin) sendHelpEmail(user *model.User, cfg *configuration, attemptCount int, triggeredAt time.Time, isTest bool) error {
	if user == nil {
		return fmt.Errorf("cannot send help email to a nil user")
	}

	if strings.TrimSpace(user.Email) == "" {
		return fmt.Errorf("user %s has no email address", user.Id)
	}

	body, err := renderHelpEmail(helpEmailData{
		ConfluenceURL: cfg.ConfluenceURL,
		AttemptCount:  attemptCount,
		WindowMinutes: cfg.WindowMinutes,
		TriggeredAt:   triggeredAt.UTC().Format(time.RFC3339),
		IsTest:        isTest,
	})
	if err != nil {
		return err
	}

	subject := cfg.EmailSubject
	if isTest {
		subject = "[TEST] " + subject
	}

	if appErr := p.API.SendMail(user.Email, subject, body); appErr != nil {
		return fmt.Errorf("send help email: %s", appErr.Error())
	}

	return nil
}

func renderHelpEmail(data helpEmailData) (string, error) {
	var out bytes.Buffer
	if err := helpEmailTemplate.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render help email: %w", err)
	}

	return out.String(), nil
}
