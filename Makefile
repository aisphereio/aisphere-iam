GO ?= go
BUF ?= buf

KERNEL_MODULE ?= github.com/aisphereio/kernel
KERNEL_VERSION ?= v0.2.1
KERNEL_LOCAL ?= ../kernel

APP_NAME ?= aisphere-iam
APP_CMD ?= ./cmd/$(APP_NAME)
CONF ?= ./configs/config.yaml
RUN_ARGS ?= -conf $(CONF)

LOCAL_BIN := $(CURDIR)/.bin
BIN_DIR := $(CURDIR)/bin
COVERPROFILE ?= coverage.out

ifeq ($(OS),Windows_NT)
LOCAL_BIN := $(CURDIR)\.bin
BIN_DIR := $(CURDIR)\bin
VERSION ?= $(shell git describe --tags --always --dirty 2>NUL || echo dev)
export PATH := $(LOCAL_BIN);$(PATH)
else
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
export PATH := $(LOCAL_BIN):$(PATH)
endif

.PHONY: help init tools tools-local check-tools api proto-check config wire generate build run test tidy verify clean

help:
	@echo "Kernel service targets:"
	@echo "  make init         install local toolchain into .bin"
	@echo "  make tools        install codegen tools into .bin"
	@echo "  make tools-local  install codegen tools from local KERNEL_LOCAL=../kernel"
	@echo "  make check-tools  check required tools in .bin"
	@echo "  make api          generate api proto code by buf.gen.yaml"
	@echo "  make proto-check  run buf lint and aisphere proto contract checks"
	@echo "  make config       generate internal config proto code if buf.gen.config.yaml exists"
	@echo "  make wire         generate dependency injection code"
	@echo "  make generate     run go generate"
	@echo "  make build        build service binary"
	@echo "  make run          run service locally"
	@echo "  make test         run all tests"
	@echo "  make tidy         run go mod tidy"
	@echo "  make verify       run api, config, wire, generate, tidy, test, build"
	@echo "  make clean        clean local artifacts"
	@echo ""
	@echo "Variables:"
	@echo "  KERNEL_MODULE=$(KERNEL_MODULE)"
	@echo "  KERNEL_VERSION=$(KERNEL_VERSION)"
	@echo "  APP_NAME=$(APP_NAME)"
	@echo "  APP_CMD=$(APP_CMD)"
	@echo "  CONF=$(CONF)"

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
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-kernel@$(KERNEL_VERSION)"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_MODULE)/cmd/buf-check-aisphere@$(KERNEL_VERSION)"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/bufbuild/buf/cmd/buf@v1.50.0"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/google/wire/cmd/wire@v0.7.0"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0"
else
	@mkdir -p $(LOCAL_BIN)
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-http@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-errors@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-authz@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-gateway@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-kernel@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/buf-check-aisphere@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/bufbuild/buf/cmd/buf@v1.50.0
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/google/wire/cmd/wire@v0.7.0
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0
endif

tools-local:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist .bin mkdir .bin"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& cd $(KERNEL_LOCAL) && $(GO) install ./cmd/protoc-gen-go-http ./cmd/protoc-gen-go-errors ./cmd/protoc-gen-go-authz ./cmd/protoc-gen-go-gateway ./cmd/protoc-gen-go-kernel ./cmd/buf-check-aisphere"
else
	@mkdir -p $(LOCAL_BIN)
	@cd $(KERNEL_LOCAL) && GOBIN=$(LOCAL_BIN) $(GO) install ./cmd/protoc-gen-go-http ./cmd/protoc-gen-go-errors ./cmd/protoc-gen-go-authz ./cmd/protoc-gen-go-gateway ./cmd/protoc-gen-go-kernel ./cmd/buf-check-aisphere
endif

check-tools:
	@$(LOCAL_BIN)/protoc-gen-go --version >/dev/null
	@$(LOCAL_BIN)/protoc-gen-go-grpc --version >/dev/null || true
	@$(LOCAL_BIN)/buf --version >/dev/null

api:
	$(BUF) generate --template buf.gen.yaml

proto-check:
	$(BUF) lint
	$(BUF) build -o - | $(LOCAL_BIN)/buf-check-aisphere

config:
	@if [ -f buf.gen.config.yaml ]; then $(BUF) generate --template buf.gen.config.yaml; fi

wire:
	@if [ -d internal/server ]; then $(GO) generate ./internal/server/...; fi

generate:
	$(GO) generate ./...

tidy:
	$(GO) mod tidy

test:
	$(GO) test ./...

build:
	$(GO) build -ldflags "-X main.Name=$(APP_NAME) -X main.Version=$(VERSION)" -o $(BIN_DIR)/$(APP_NAME) $(APP_CMD)

run:
	$(GO) run $(APP_CMD) $(RUN_ARGS)

verify: api proto-check config wire generate tidy test build

clean:
	$(GO) clean
	@rm -rf $(LOCAL_BIN) $(BIN_DIR) $(COVERPROFILE)
