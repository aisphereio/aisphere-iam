GO ?= go
BUF ?= buf

KERNEL_MODULE ?= github.com/aisphereio/kernel
KERNEL_VERSION ?= __KERNEL_VERSION__

APP_NAME ?= server
APP_CMD ?= ./cmd/$(APP_NAME)
CONF ?= ./configs/config.yaml
RUN_ARGS ?= -conf $(CONF)
LOCAL_BIN := $(CURDIR)/.bin
BIN_DIR := $(CURDIR)/bin

ifeq ($(OS),Windows_NT)
LOCAL_BIN := $(CURDIR)\.bin
BIN_DIR := $(CURDIR)\bin
VERSION ?= $(shell git describe --tags --always --dirty 2>NUL || echo dev)
export PATH := $(LOCAL_BIN);$(PATH)
else
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
export PATH := $(LOCAL_BIN):$(PATH)
endif

.PHONY: help tools check-tools api proto-check build run test tidy verify clean

help:
	@echo "Kernel MVP service targets: tools api proto-check test run build verify clean"

tools:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist .bin mkdir .bin"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-http@$(KERNEL_VERSION)"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-errors@$(KERNEL_VERSION)"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0"
	@cmd /c "set GOBIN=$(LOCAL_BIN)&& $(GO) install github.com/bufbuild/buf/cmd/buf@v1.50.0"
else
	@mkdir -p $(LOCAL_BIN)
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
	@GOBIN=$(LOCAL_BIN) $(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-http@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install $(KERNEL_MODULE)/cmd/protoc-gen-go-errors@$(KERNEL_VERSION)
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.29.0
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.29.0
	@GOBIN=$(LOCAL_BIN) $(GO) install github.com/bufbuild/buf/cmd/buf@v1.50.0
endif

check-tools:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist .bin\buf.exe echo missing .bin\buf.exe && exit /b 1"
else
	@test -x "$(LOCAL_BIN)/buf" || (echo "missing $(LOCAL_BIN)/buf"; exit 1)
endif

api: check-tools
ifeq ($(OS),Windows_NT)
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& .bin\buf.exe generate --template buf.gen.yaml"
else
	@PATH="$(LOCAL_BIN):$$PATH" $(LOCAL_BIN)/buf generate --template buf.gen.yaml
endif

proto-check: check-tools
ifeq ($(OS),Windows_NT)
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& .bin\buf.exe lint"
	@cmd /c "set PATH=$(LOCAL_BIN);%PATH%&& .bin\buf.exe build -o .bin\layout-descriptor.bin"
else
	@PATH="$(LOCAL_BIN):$$PATH" $(LOCAL_BIN)/buf lint
	@PATH="$(LOCAL_BIN):$$PATH" $(LOCAL_BIN)/buf build -o $(LOCAL_BIN)/layout-descriptor.bin
endif

run:
	$(GO) run $(APP_CMD) $(RUN_ARGS)

test:
	$(GO) test ./...

build:
ifeq ($(OS),Windows_NT)
	@cmd /c "if not exist bin mkdir bin"
	$(GO) build -ldflags "-X main.Version=$(VERSION)" -o bin\$(APP_NAME).exe $(APP_CMD)
else
	@mkdir -p bin
	$(GO) build -ldflags "-X main.Version=$(VERSION)" -o bin/$(APP_NAME) $(APP_CMD)
endif

tidy:
	$(GO) mod tidy

verify: api proto-check tidy test build

clean:
ifeq ($(OS),Windows_NT)
	@cmd /c "if exist .bin rmdir /s /q .bin"
	@cmd /c "if exist bin rmdir /s /q bin"
else
	rm -rf $(LOCAL_BIN) bin
endif
