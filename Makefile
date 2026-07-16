GO ?= go
BUF ?= buf
KUBECTL ?= kubectl

KERNEL_MODULE ?= github.com/aisphereio/kernel
KERNEL_VERSION ?= v0.4.15
KERNEL_LOCAL ?= ../kernel
BASE_REF ?= main
OPENAPI_VERSION ?= v1

APP_NAME ?= aisphere-iam
APP_CMD ?= ./cmd/$(APP_NAME)
CONF ?= ./configs/config.yaml
RUN_ARGS ?= -conf $(CONF)

LOCAL_BIN := $(CURDIR)/.bin
BIN_DIR := $(CURDIR)/bin
COVERPROFILE ?= coverage.out
DEPLOY_DIR ?= deploy
GENERATED_DEPLOY_DIR ?= $(DEPLOY_DIR)/generated
KUBE_NAMESPACE ?= aisphere
SPICEDB_SCHEMA ?= ./configs/spicedb/aisphere.schema.zed
SPICEDB_SCHEMA_CONFIGMAP ?= aisphere-iam-spicedb-schema
PERMISSION_MANIFEST ?= ./configs/resource/defaults.yaml

ifeq ($(OS),Windows_NT)
LOCAL_BIN := $(CURDIR)\.bin
BIN_DIR := $(CURDIR)\bin
GENERATED_DEPLOY_DIR := $(DEPLOY_DIR)\generated
VERSION ?= $(shell git describe --tags --always --dirty 2>NUL || echo dev)
export PATH := $(LOCAL_BIN);$(PATH)
else
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
export PATH := $(LOCAL_BIN):$(PATH)
endif

.PHONY: help init tools tools-local check-tools api api-generate deploy spicedb-schema-configmap deploy-apply proto-check breaking-check openapi-check contract-check generated-check contract-bundle permission-manifest-check config wire generate build docker run test tidy verify clean

help:
	@echo "Aisphere IAM targets:"
	@echo "  make tools                   install released Kernel v$(KERNEL_VERSION) codegen tools into .bin"
	@echo "  make tools-local             install codegen tools from local KERNEL_LOCAL=../kernel"
	@echo "  make api                     generate API proto code + openapi-check by buf.gen.yaml"
	@echo "  make api-generate            generate API proto code only (skip openapi-check)"
	@echo "  make deploy                  generate Gateway API manifests under deploy/generated"
	@echo "  make spicedb-schema-configmap generate/apply SpiceDB schema ConfigMap from $(SPICEDB_SCHEMA)"
	@echo "  make deploy-apply            generate and apply app manifests plus generated Gateway API manifests"
	@echo "  make proto-check             run buf lint and aisphere proto contract checks"
	@echo "  make breaking-check          reject protobuf changes incompatible with BASE_REF=$(BASE_REF)"
	@echo "  make openapi-check           normalize and validate the tracked IAM OpenAPI contract"
	@echo "  make contract-check          run protobuf, compatibility, generation, and OpenAPI gates"
	@echo "  make generated-check         verify generated files are committed (no drift)"
	@echo "  make contract-bundle         build contract bundle (swagger + lock) under dist/api-contract/"
	@echo "  make permission-manifest-check validate IAM permission manifest against SpiceDB schema"
	@echo "  make traceability-check      validate REQ->ART->TC traceability chain"
	@echo "  make verify                  run contract-check, deploy, checks, config, wire, generate, tidy, test, build, traceability"
	@echo ""
	@echo "Variables: KERNEL_VERSION=$(KERNEL_VERSION) APP_NAME=$(APP_NAME) CONF=$(CONF) KUBE_NAMESPACE=$(KUBE_NAMESPACE)"

init: tools

tools:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist .bin mkdir .bin"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-http@$(KERNEL_VERSION)"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-errors@$(KERNEL_VERSION)"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-authz@$(KERNEL_VERSION)"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-gateway@$(KERNEL_VERSION)"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-deploy@$(KERNEL_VERSION)"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-kernel@$(KERNEL_VERSION)"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_MODULE)/cmd/buf-check-aisphere@$(KERNEL_VERSION)"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/bufbuild/buf/cmd/buf@v1.50.0"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/google/wire/cmd/wire@v0.7.0"
else
	@mkdir -p $(LOCAL_BIN)
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-http@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-errors@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-authz@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-gateway@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-deploy@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-kernel@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/buf-check-aisphere@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/bufbuild/buf/cmd/buf@v1.50.0
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/google/wire/cmd/wire@v0.7.0
endif

tools-local:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist .bin mkdir .bin"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-http"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-errors"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-authz"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-gateway"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-deploy"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-kernel"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_LOCAL)/cmd/buf-check-aisphere"
else
	@mkdir -p $(LOCAL_BIN)
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-http
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-errors
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-authz
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-gateway
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-deploy
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_LOCAL)/cmd/protoc-gen-go-kernel
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_LOCAL)/cmd/buf-check-aisphere
endif

check-tools:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist .bin\buf.exe echo missing .bin\buf.exe && exit /b 1"
	@cmd /c "if not exist .bin\protoc-gen-go.exe echo missing .bin\protoc-gen-go.exe && exit /b 1"
	@cmd /c "if not exist .bin\protoc-gen-go-deploy.exe echo missing .bin\protoc-gen-go-deploy.exe && exit /b 1"
	@cmd /c "if not exist .bin\buf-check-aisphere.exe echo missing .bin\buf-check-aisphere.exe && exit /b 1"
else
	@test -x "$(LOCAL_BIN)/buf" || (echo "missing $(LOCAL_BIN)/buf"; exit 1)
	@test -x "$(LOCAL_BIN)/protoc-gen-go" || (echo "missing $(LOCAL_BIN)/protoc-gen-go"; exit 1)
	@test -x "$(LOCAL_BIN)/protoc-gen-go-deploy" || (echo "missing $(LOCAL_BIN)/protoc-gen-go-deploy"; exit 1)
	@test -x "$(LOCAL_BIN)/buf-check-aisphere" || (echo "missing $(LOCAL_BIN)/buf-check-aisphere"; exit 1)
endif

api: api-generate openapi-check

api-generate: check-tools
ifeq ($(OS),Windows_NT)
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& .bin\buf.exe generate --template buf.gen.yaml"
else
	@PATH="$(LOCAL_BIN):$$PATH" $(LOCAL_BIN)/buf generate --template buf.gen.yaml
endif

deploy: check-tools
ifeq ($(OS),Windows_NT)
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& if exist $(GENERATED_DEPLOY_DIR) rmdir /s /q $(GENERATED_DEPLOY_DIR)"
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& .bin\buf.exe generate --template buf.gen.deploy.yaml"
else
	@rm -rf $(GENERATED_DEPLOY_DIR)
	@PATH="$(LOCAL_BIN):$$PATH" $(LOCAL_BIN)/buf generate --template buf.gen.deploy.yaml
endif
	@echo "✓ generated Gateway API manifests under $(GENERATED_DEPLOY_DIR)"

spicedb-schema-configmap:
ifeq ($(OS),Windows_NT)
	@cmd /c "$(KUBECTL) create configmap $(SPICEDB_SCHEMA_CONFIGMAP) -n $(KUBE_NAMESPACE) --from-file=aisphere.schema.zed=$(SPICEDB_SCHEMA) --dry-run=client -o yaml | $(KUBECTL) apply -f -"
else
	$(KUBECTL) create configmap $(SPICEDB_SCHEMA_CONFIGMAP) -n $(KUBE_NAMESPACE) --from-file=aisphere.schema.zed=$(SPICEDB_SCHEMA) --dry-run=client -o yaml | $(KUBECTL) apply -f -
endif

deploy-apply: deploy
ifeq ($(OS),Windows_NT)
	@cmd /c "$(KUBECTL) get secret aisphere-iam-secrets -n $(KUBE_NAMESPACE) >nul 2>nul || (echo ERROR: aisphere-iam-secrets not found. Create it first: kubectl create secret generic aisphere-iam-secrets -n $(KUBE_NAMESPACE) --from-literal=postgres-dsn=... && exit /b 1)"
	@cmd /c "$(KUBECTL) apply -f $(DEPLOY_DIR)\namespace.yaml"
	@cmd /c "$(KUBECTL) create configmap $(SPICEDB_SCHEMA_CONFIGMAP) -n $(KUBE_NAMESPACE) --from-file=aisphere.schema.zed=$(SPICEDB_SCHEMA) --dry-run=client -o yaml | $(KUBECTL) apply -f -"
	@cmd /c "$(KUBECTL) apply -f $(DEPLOY_DIR)\configmap.yaml"
	@cmd /c "$(KUBECTL) apply -f $(DEPLOY_DIR)\service.yaml"
	@cmd /c "$(KUBECTL) apply -f $(DEPLOY_DIR)\networkpolicy.yaml"
	@cmd /c "$(KUBECTL) apply -f $(DEPLOY_DIR)\deployment.yaml"
	@cmd /c "if exist $(GENERATED_DEPLOY_DIR) ($(KUBECTL) apply -R -f $(GENERATED_DEPLOY_DIR)) else (echo generated deploy dir missing && exit /b 1)"
else
	@kubectl get secret aisphere-iam-secrets -n $(KUBE_NAMESPACE) >/dev/null 2>&1 || (echo "ERROR: aisphere-iam-secrets not found in namespace $(KUBE_NAMESPACE). Create it first:"; echo "  kubectl create secret generic aisphere-iam-secrets -n $(KUBE_NAMESPACE) --from-literal=postgres-dsn=..."; exit 1)
	$(KUBECTL) apply -f $(DEPLOY_DIR)/namespace.yaml
	$(KUBECTL) create configmap $(SPICEDB_SCHEMA_CONFIGMAP) -n $(KUBE_NAMESPACE) --from-file=aisphere.schema.zed=$(SPICEDB_SCHEMA) --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) apply -f $(DEPLOY_DIR)/configmap.yaml
	$(KUBECTL) apply -f $(DEPLOY_DIR)/service.yaml
	$(KUBECTL) apply -f $(DEPLOY_DIR)/networkpolicy.yaml
	$(KUBECTL) apply -f $(DEPLOY_DIR)/deployment.yaml
	@test -d $(GENERATED_DEPLOY_DIR) || (echo "$(GENERATED_DEPLOY_DIR) missing; run make deploy"; exit 1)
	$(KUBECTL) apply -R -f $(GENERATED_DEPLOY_DIR)
endif

proto-check: check-tools
ifeq ($(OS),Windows_NT)
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& .bin\buf.exe lint"
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& .bin\buf.exe build -o - | .bin\buf-check-aisphere.exe"
else
	@PATH="$(LOCAL_BIN):$$PATH" $(LOCAL_BIN)/buf lint
	@PATH="$(LOCAL_BIN):$$PATH" $(LOCAL_BIN)/buf build -o - | $(LOCAL_BIN)/buf-check-aisphere
endif

breaking-check: check-tools
ifeq ($(OS),Windows_NT)
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& .bin\buf.exe breaking --against .git#branch=$(BASE_REF)"
else
	@PATH="$(LOCAL_BIN):$$PATH" $(LOCAL_BIN)/buf breaking --against '.git#branch=$(BASE_REF)'
endif

openapi-check:
	$(GO) run ./cmd/openapi-contract-check --input docs/openapi/aisphere.swagger.json --output docs/openapi/aisphere.swagger.json --title "Aisphere IAM API" --version "$(OPENAPI_VERSION)"

contract-check: proto-check breaking-check api

generated-check: api
	git diff --exit-code -- api docs/openapi

CONTRACT_BUNDLE_DIR ?= dist/api-contract

contract-bundle: api
	@mkdir -p $(CONTRACT_BUNDLE_DIR)
	cp docs/openapi/aisphere.swagger.json $(CONTRACT_BUNDLE_DIR)/
	@GIT_SHA=$$(git rev-parse HEAD 2>/dev/null || echo "unknown"); \
	GIT_REF=$$(git symbolic-ref --short HEAD 2>/dev/null || echo "unknown"); \
	SHA256=$$(sha256sum $(CONTRACT_BUNDLE_DIR)/aisphere.swagger.json 2>/dev/null | cut -d' ' -f1 || \
	         certutil -hashfile $(CONTRACT_BUNDLE_DIR)/aisphere.swagger.json SHA256 2>/dev/null | findstr /v ":" | findstr /v "SHA" | tr -d ' \r\n' || echo "unknown"); \
	printf '{\n  "repository": "https://github.com/aisphereio/aisphere-iam.git",\n  "git_sha": "%s",\n  "ref": "%s",\n  "sha256": "%s",\n  "kernel_version": "%s",\n  "generator": "protoc-gen-openapiv2@v2.29.0"\n}\n' \
		"$$GIT_SHA" "$$GIT_REF" "$$SHA256" "$(KERNEL_VERSION)" > $(CONTRACT_BUNDLE_DIR)/contract-lock.json
	@echo "✓ contract bundle generated under $(CONTRACT_BUNDLE_DIR)/"

permission-manifest-check:
	$(GO) run ./cmd/permission-manifest-check --manifest $(PERMISSION_MANIFEST) --schema $(SPICEDB_SCHEMA)

config: check-tools
ifeq ($(OS),Windows_NT)
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& if exist buf.gen.config.yaml (.bin\buf.exe generate --template buf.gen.config.yaml) else (echo buf.gen.config.yaml not found; skip config)"
else
	@if [ -f buf.gen.config.yaml ]; then PATH="$(LOCAL_BIN):$$PATH" $(LOCAL_BIN)/buf generate --template buf.gen.config.yaml; else echo "buf.gen.config.yaml not found; skip config"; fi
endif

wire:
	@if [ -d internal/server ]; then $(GO) generate ./internal/server/...; else echo "internal/server not found; skip wire"; fi

generate:
	$(GO) generate ./...

tidy:
	$(GO) mod tidy

test:
	$(GO) test ./... -coverprofile=$(COVERPROFILE)

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags "-s -w -X main.Name=$(APP_NAME) -X main.Version=$(VERSION)" -o $(BIN_DIR)/$(APP_NAME) $(APP_CMD)

docker:
	docker build --build-arg VERSION=$(VERSION) -t $(APP_NAME):$(VERSION) .

run:
	$(GO) run $(APP_CMD) $(RUN_ARGS)

traceability-check:
	$(GO) run ./cmd/traceability-check/ $(if $(STRICT),--strict)

verify: contract-check deploy permission-manifest-check config wire generate tidy test build traceability-check

clean:
ifeq ($(OS),Windows_NT)
	@cmd /c "if exist .bin rmdir /s /q .bin"
	@cmd /c "if exist bin rmdir /s /q bin"
	@cmd /c "if exist $(GENERATED_DEPLOY_DIR) rmdir /s /q $(GENERATED_DEPLOY_DIR)"
	@cmd /c "if exist $(COVERPROFILE) del /q $(COVERPROFILE)"
else
	rm -rf $(LOCAL_BIN) $(BIN_DIR) $(GENERATED_DEPLOY_DIR) $(COVERPROFILE)
endif
