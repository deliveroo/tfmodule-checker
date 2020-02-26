PROJECTNAME := $(shell basename "$(PWD)")

SHELL = bash
.DEFAULT_GOAL := help

# Go related variables.
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin

# Redirect error output to a file, so we can show it in development mode.
LOG_FILE=/tmp/.$(PROJECTNAME).log

# PID file will keep the process ID of the server.
PID := /tmp/.$(PROJECTNAME).pid

# --silent drops the need to prepend `@` to suppress command output.
MAKEFLAGS += --silent

# GOPRIVATE because GOPROXY is not set up internally
GOPRIVATE=github.com/deliveroo,github.com/go-critic,github.com/golangci

##################################################################

.PHONY: help
help: ## This text.
	grep -E '^[/a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: proj
proj: ## initialise new git project
	[[ ! -d .circleci ]] && mkdir .circleci
	touch README.md .gitignore .dockerignore .envrc .circleci/config.yml
	if [[ ! -d .git ]]; then git init; fi

.PHONY: format
format: ## Reformat code to standard
	go fmt ./...

.PHONY: build-no-test
build-no-test: ## Build project without tests.
	GOBIN=$(GOBIN) go build  ./...

.PHONY: build
build: test ## Build project.
	GOBIN=$(GOBIN) go build ./...

.PHONY: install
install: test ## Install binaries to $(GOBIN)
	GOBIN=$(GOBIN) go install ./cmd/...

.PHONY: clean
clean: ## Clean build files and artifacts.
	GOBIN=$(GOBIN) go clean ./...
	rm -rfv $(GOBIN) $(LOG_FILE)
	rm -f *~ */*~ */*/*~

.PHONY: test
test:
	go test ./...
