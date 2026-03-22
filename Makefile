PLUGIN_ID ?= com.mattermost.login-help-mailer
VERSION ?= 0.1.0
GO ?= go

DIST_DIR := dist
PLUGIN_DIR := $(DIST_DIR)/$(PLUGIN_ID)
SERVER_DIST := $(PLUGIN_DIR)/server/dist

.PHONY: build bundle clean

build:
	mkdir -p $(SERVER_DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -o $(SERVER_DIST)/plugin-linux-amd64 ./server
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -o $(SERVER_DIST)/plugin-linux-arm64 ./server
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -o $(SERVER_DIST)/plugin-darwin-amd64 ./server
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -o $(SERVER_DIST)/plugin-darwin-arm64 ./server
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -o $(SERVER_DIST)/plugin-windows-amd64.exe ./server

bundle: clean build
	mkdir -p $(PLUGIN_DIR)/assets
	cp plugin.json $(PLUGIN_DIR)/
	cp assets/icon.svg $(PLUGIN_DIR)/assets/icon.svg
	tar -czf $(DIST_DIR)/$(PLUGIN_ID)-$(VERSION).tar.gz -C $(DIST_DIR) $(PLUGIN_ID)

clean:
	rm -rf $(DIST_DIR)
