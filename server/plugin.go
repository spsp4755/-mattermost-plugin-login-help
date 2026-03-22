package main

import (
	"fmt"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

type LoginHelpPlugin struct {
	plugin.MattermostPlugin

	configurationLock sync.RWMutex
	configuration     *configuration

	watcherLock sync.Mutex
	watcher     *auditWatcher

	validationLock sync.RWMutex
	validationErr  string
}

func (p *LoginHelpPlugin) OnActivate() error {
	p.reconcileWatcher()
	return nil
}

func (p *LoginHelpPlugin) OnDeactivate() error {
	p.stopWatcher()
	return nil
}

func (p *LoginHelpPlugin) OnConfigurationChange() error {
	cfg := &configuration{}
	if err := p.API.LoadPluginConfiguration(cfg); err != nil {
		return fmt.Errorf("load plugin configuration: %w", err)
	}

	cfg.setDefaults()
	p.setConfiguration(cfg)
	p.reconcileWatcher()
	return nil
}

func (p *LoginHelpPlugin) UserHasLoggedIn(_ *plugin.Context, user *model.User) {
	cfg := p.getConfiguration()
	if !cfg.ResetOnSuccess {
		return
	}

	if cfg.OnlyLocalAccounts && !isLocalMattermostAccount(user) {
		return
	}

	if err := newPluginStore(p.API).deleteFailureState(user.Id); err != nil {
		p.API.LogWarn("Failed to clear login failure state after successful login", "user_id", user.Id, "error", err.Error())
	}
}

func (p *LoginHelpPlugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		cfg := &configuration{}
		cfg.setDefaults()
		return cfg
	}

	return p.configuration.clone()
}

func (p *LoginHelpPlugin) setConfiguration(cfg *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()
	p.configuration = cfg.clone()
}

func (p *LoginHelpPlugin) getValidationError() string {
	p.validationLock.RLock()
	defer p.validationLock.RUnlock()
	return p.validationErr
}

func (p *LoginHelpPlugin) setValidationError(msg string) {
	p.validationLock.Lock()
	defer p.validationLock.Unlock()
	p.validationErr = msg
}

func (p *LoginHelpPlugin) reconcileWatcher() {
	cfg := p.getConfiguration()
	p.stopWatcher()

	if err := cfg.validate(); err != nil {
		p.setValidationError(err.Error())
		p.API.LogError("Login Help Mailer watcher is disabled because configuration is invalid", "error", err.Error())
		return
	}

	p.setValidationError("")
	if !cfg.Enabled {
		p.API.LogInfo("Login Help Mailer watcher is disabled by configuration")
		return
	}

	watcher := newAuditWatcher(p, cfg)
	p.watcherLock.Lock()
	p.watcher = watcher
	p.watcherLock.Unlock()

	p.API.LogInfo("Starting Login Help Mailer watcher", "audit_log_path", cfg.AuditLogPath)
	watcher.start()
}

func (p *LoginHelpPlugin) stopWatcher() {
	p.watcherLock.Lock()
	watcher := p.watcher
	p.watcher = nil
	p.watcherLock.Unlock()

	if watcher != nil {
		watcher.stop()
	}
}

func (p *LoginHelpPlugin) getWatcherStatus() watcherStatus {
	p.watcherLock.Lock()
	watcher := p.watcher
	p.watcherLock.Unlock()

	if watcher == nil {
		cfg := p.getConfiguration()
		return watcherStatus{
			Enabled:   cfg.Enabled,
			FilePath:  cfg.AuditLogPath,
			Running:   false,
			LastError: p.getValidationError(),
		}
	}

	status := watcher.snapshot()
	if status.LastError == "" {
		status.LastError = p.getValidationError()
	}

	return status
}

func main() {
	plugin.ClientMain(&LoginHelpPlugin{})
}
