GO ?= go
KUBECTL ?= kubectl

KERNEL_MODULE ?= github.com/aisphereio/kernel
KERNEL_VERSION ?= v0.4.2
KERNEL_LOCAL ?= ../kernel

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

ifeq ($(OS),Windows_NT)
LOCAL_BIN := $(CURDIR)\.bin
BIN_DIR := $(CURDIR)\bin
VERSION ?= $(shell git describe --tags --always --dirty 2>NUL || echo dev)
export PATH := $(LOCAL_BIN);$(PATH)
else
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
export PATH := $(LOCAL_BIN):$(PATH)
endif

.PHONY: help init tools tools-local check-tools api deploy spicedb-schema-configmap deploy-apply proto-check config wire generate build docker run test tidy verify clean

help:
	@echo "Aisphere IAM targets:"
	@echo "  make tools                   install released Kernel $(KERNEL_VERSION) codegen tools into .bin"
	@echo "  make tools-local             install codegen tools from local KERNEL_LOCAL=../kernel"
	@echo "  make api                     generate API proto code by buf.gen.yaml"
	@echo "  make deploy                  generate Gateway API manifests under deploy/generated"
	@echo "  make spicedb-schema-configmap generate/apply SpiceDB schema ConfigMap"
	@echo "  make deploy-apply            generate and apply app plus Gateway API manifests"
	@echo "  make proto-check             run buf lint and Aisphere contract checks"
	@echo "  make verify                  run generation, checks, tests and build"
	@echo ""
	@echo "Variables: KERNEL_VERSION=$(KERNEL_VERSION) APP_NAME=$(APP_NAME) CONF=$(CONF) KUBE_NAMESPACE=$(KUBE_NAMESPACE)"

init: tools

ifeq ($(OS),Windows_NT)
define INSTALL_TOOL
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(1)"
endef
else
define INSTALL_TOOL
	@GOBIN=$(LOCAL_BIN) $(GO) install $(1)
endef
endif

tools:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist .bin mkdir .bin"
else
	@mkdir -p $(LOCAL_BIN)
endif
	$(call INSTALL_TOOL,google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11)
	$(call INSTALL_TOOL,google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1)
	$(call INSTALL_TOOL,$(KERNEL_MODULE)/cmd/protoc-gen-go-http@$(KERNEL_VERSION))
	$(call INSTALL_TOOL,$(KERNEL_MODULE)/cmd/protoc-gen-go-errors@$(KERNEL_VERSION))
	$(call INSTALL_TOOL,$(KERNEL_MODULE)/cmd/protoc-gen-go-authz@$(KERNEL_VERSION))
	$(call INSTALL_TOOL,$(KERNEL_MODULE)/cmd/protoc-gen-go-gateway@$(KERNEL_VERSION))
	$(call INSTALL_TOOL,$(KERNEL_MODULE)/cmd/protoc-gen-go-deploy@$(KERNEL_VERSION))
	$(call INSTALL_TOOL,$(KERNEL_MODULE)/cmd/protoc-gen-go-kernel@$(KERNEL_VERSION))
	$(call INSTALL_TOOL,$(KERNEL_MODULE)/cmd/buf-check-aisphere@$(KERNEL_VERSION))
	$(call INSTALL_TOOL,github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0)
	$(call INSTALL_TOOL,github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0)
	$(call INSTALL_TOOL,github.com/bufbuild/buf/cmd/buf@v1.50.0)
	$(call INSTALL_TOOL,github.com/google/wire/cmd/wire@v0.7.0)

tools-local:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist .bin mkdir .bin"
else
	@mkdir -p $(LOCAL_BIN)
endif
	$(call INSTALL_TOOL,google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11)
	$(call INSTALL_TOOL,google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1)
	$(call INSTALL_TOOL,$(KERNEL_LOCAL)/cmd/protoc-gen-go-http)
	$(call INSTALL_TOOL,$(KERNEL_LOCAL)/cmd/protoc-gen-go-errors)
	$(call INSTALL_TOOL,$(KERNEL_LOCAL)/cmd/protoc-gen-go-authz)
	$(call INSTALL_TOOL,$(KERNEL_LOCAL)/cmd/protoc-gen-go-gateway)
	$(call INSTALL_TOOL,$(KERNEL_LOCAL)/cmd/protoc-gen-go-deploy)
	$(call INSTALL_TOOL,$(KERNEL_LOCAL)/cmd/protoc-gen-go-kernel)
	$(call INSTALL_TOOL,$(KERNEL_LOCAL)/cmd/buf-check-aisphere)

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

api: check-tools
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
	@echo "generated Gateway API manifests under $(GENERATED_DEPLOY_DIR)"

spicedb-schema-configmap:
	$(KUBECTL) create configmap $(SPICEDB_SCHEMA_CONFIGMAP) -n $(KUBE_NAMESPACE) --from-file=aisphere.schema.zed=$(SPICEDB_SCHEMA) --dry-run=client -o yaml | $(KUBECTL) apply -f -

deploy-apply: deploy
	@$(KUBECTL) get secret aisphere-iam-secrets -n $(KUBE_NAMESPACE) >/dev/null 2>&1 || (echo "ERROR: aisphere-iam-secrets not found in namespace $(KUBE_NAMESPACE)"; exit 1)
	$(KUBECTL) apply -f $(DEPLOY_DIR)/namespace.yaml
	$(KUBECTL) create configmap $(SPICEDB_SCHEMA_CONFIGMAP) -n $(KUBE_NAMESPACE) --from-file=aisphere.schema.zed=$(SPICEDB_SCHEMA) --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) apply -f $(DEPLOY_DIR)/configmap.yaml
	$(KUBECTL) apply -f $(DEPLOY_DIR)/service.yaml
	$(KUBECTL) apply -f $(DEPLOY_DIR)/networkpolicy.yaml
	$(KUBECTL) apply -f $(DEPLOY_DIR)/deployment.yaml
	@test -d $(GENERATED_DEPLOY_DIR) || (echo "$(GENERATED_DEPLOY_DIR) missing; run make deploy"; exit 1)
	$(KUBECTL) apply -R -f $(GENERATED_DEPLOY_DIR)

proto-check: check-tools
ifeq ($(OS),Windows_NT)
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& .bin\buf.exe lint"
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& .bin\buf.exe build -o - | .bin\buf-check-aisphere.exe"
else
	@PATH="$(LOCAL_BIN):$$PATH" $(LOCAL_BIN)/buf lint
	@PATH="$(LOCAL_BIN):$$PATH" $(LOCAL_BIN)/buf build -o - | $(LOCAL_BIN)/buf-check-aisphere
endif

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
	$(GO) test ./...

build:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist bin mkdir bin"
	$(GO) build -ldflags "-X main.Name=$(APP_NAME) -X main.Version=$(VERSION)" -o bin\$(APP_NAME).exe $(APP_CMD)
else
	@mkdir -p bin
	$(GO) build -ldflags "-X main.Name=$(APP_NAME) -X main.Version=$(VERSION)" -o bin/$(APP_NAME) $(APP_CMD)
endif

docker:
	docker build --build-arg VERSION=$(VERSION) -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .

run:
	$(GO) run $(APP_CMD) $(RUN_ARGS)

verify: api deploy proto-check config wire generate tidy test build

clean:
ifeq ($(OS),Windows_NT)
	@cmd /c "if exist .bin rmdir /s /q .bin"
	@cmd /c "if exist bin rmdir /s /q bin"
	@cmd /c "if exist $(COVERPROFILE) del /f /q $(COVERPROFILE)"
	@cmd /c "if exist coverage.html del /f /q coverage.html"
else
	rm -rf $(LOCAL_BIN)
	rm -rf $(BIN_DIR)
	rm -rf $(GENERATED_DEPLOY_DIR)
	rm -f $(COVERPROFILE) coverage.html
endif
