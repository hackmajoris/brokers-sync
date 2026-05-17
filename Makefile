AWS_PROFILE  ?= hackmajoris-aws
AWS_REGION   ?= eu-central-1
# Empty AWS_PROFILE (e.g. in CI) → no --profile flag; OIDC/env credentials are used instead.
PROFILE_ARG   = $(if $(AWS_PROFILE),--profile $(AWS_PROFILE))
AWS_ACCOUNT  ?= $(shell aws sts get-caller-identity $(PROFILE_ARG) --query Account --output text)

.DEFAULT_GOAL := help

.PHONY: help \
        dev build build-web build-server run \
        cdk-diff cdk-deploy cdk-destroy \
        cdk-bootstrap ecr-setup

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} \
		/^## ====/ {sub(/^## ==== /, ""); sub(/ ====$$/, ""); printf "\n\033[1m%s\033[0m\n", $$0; next} \
		/^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## ==== Local development ====

dev: ## Run Go server and Vite dev server together
	@trap 'kill 0' EXIT; \
	go run ./cmd/server -data data -web web/dist & \
	cd web && npm run dev

build: build-web build-server ## Build web and server

build-web: ## Build the React frontend
	cd web && npm run build

build-server: ## Build the Go server binary
	go build -o bin/server ./cmd/server

run: build ## Build, then run the server locally
	./bin/server -data data -web web/dist

## ==== AWS / CDK ====

cdk-diff: ## Show pending CDK changes for all stacks
	cd infra && CDK_DEFAULT_ACCOUNT=$(AWS_ACCOUNT) CDK_DEFAULT_REGION=$(AWS_REGION) \
		cdk diff --all $(PROFILE_ARG)

cdk-deploy: build-web ## Build web and deploy all CDK stacks
	cd infra && CDK_DEFAULT_ACCOUNT=$(AWS_ACCOUNT) CDK_DEFAULT_REGION=$(AWS_REGION) \
		cdk deploy --all $(PROFILE_ARG) --require-approval never --outputs-file cdk-outputs.json

cdk-destroy: ## Destroy all CDK stacks
	cd infra && CDK_DEFAULT_ACCOUNT=$(AWS_ACCOUNT) CDK_DEFAULT_REGION=$(AWS_REGION) \
		cdk destroy --all $(PROFILE_ARG) --force

## ==== One-time setup ====

cdk-bootstrap: ## Bootstrap CDK in this account/region
	cd infra && cdk bootstrap aws://$(AWS_ACCOUNT)/$(AWS_REGION) $(PROFILE_ARG)

# Run once after bootstrapping a new environment to cap the CDK asset ECR repo at 2 images.
ecr-setup: ## Cap CDK asset ECR repo at 2 images
	aws ecr put-lifecycle-policy \
		--repository-name cdk-hnb659fds-container-assets-$(AWS_ACCOUNT)-$(AWS_REGION) \
		--lifecycle-policy-text '{"rules":[{"rulePriority":1,"description":"keep last 2 images","selection":{"tagStatus":"any","countType":"imageCountMoreThan","countNumber":2},"action":{"type":"expire"}}]}' \
		$(PROFILE_ARG) \
		--region $(AWS_REGION)
