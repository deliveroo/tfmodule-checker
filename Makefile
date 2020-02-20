SHELL = bash
.DEFAULT_GOAL := help
.PHONY: help clean proj purge setup


help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

proj: ## Create new empty project
	[[ ! -d .circleci ]] && mkdir .circleci
	touch README.md .gitignore .dockerignore .envrc .circleci/config.yml
	if [[ ! -d .git ]]; then git init; fi

