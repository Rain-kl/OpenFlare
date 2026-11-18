#!/bin/sh

# execute this first
# go install github.com/swaggo/swag/cmd/swag@latest

swag init -d cmd/server,internal -g main.go -o docs --parseDependency --parseInternal --exclude fast-note-sync-service-source
