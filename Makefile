AWS_PROFILE  ?= hackmajoris-aws
AWS_REGION   ?= eu-central-1
# Empty AWS_PROFILE (e.g. in CI) → no --profile flag; OIDC/env credentials are used instead.
PROFILE_ARG   = $(if $(AWS_PROFILE),--profile $(AWS_PROFILE))
AWS_ACCOUNT  ?= $(shell aws sts get-caller-identity $(PROFILE_ARG) --query Account --output text)

.PHONY: dev build build-web build-server run \
        cdk-bootstrap cdk-deploy cdk-destroy cdk-diff

dev:
	@trap 'kill 0' EXIT; \
	go run ./cmd/server -data data -web web/dist & \
	cd web && npm run dev

build: build-web build-server

build-web:
	cd web && npm run build

build-server:
	go build -o bin/server ./cmd/server

run: build
	./bin/server -data data -web web/dist

cdk-bootstrap:
	cd infra && cdk bootstrap aws://$(AWS_ACCOUNT)/$(AWS_REGION) $(PROFILE_ARG)
	@echo "Setting ECR lifecycle policy (keep 2 images)..."
	aws ecr put-lifecycle-policy \
		--repository-name cdk-hnb659fds-container-assets-$(AWS_ACCOUNT)-$(AWS_REGION) \
		--lifecycle-policy-text '{"rules":[{"rulePriority":1,"description":"keep last 2 images","selection":{"tagStatus":"any","countType":"imageCountMoreThan","countNumber":2},"action":{"type":"expire"}}]}' \
		$(PROFILE_ARG) \
		--region $(AWS_REGION)

cdk-diff:
	cd infra && CDK_DEFAULT_ACCOUNT=$(AWS_ACCOUNT) CDK_DEFAULT_REGION=$(AWS_REGION) \
		cdk diff --all $(PROFILE_ARG)

cdk-deploy: build-web
	cd infra && CDK_DEFAULT_ACCOUNT=$(AWS_ACCOUNT) CDK_DEFAULT_REGION=$(AWS_REGION) \
		cdk deploy --all $(PROFILE_ARG) --require-approval never --outputs-file cdk-outputs.json

cdk-destroy:
	cd infra && CDK_DEFAULT_ACCOUNT=$(AWS_ACCOUNT) CDK_DEFAULT_REGION=$(AWS_REGION) \
		cdk destroy --all $(PROFILE_ARG) --force
