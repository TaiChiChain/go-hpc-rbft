SHELL := /bin/bash

GO_BIN = go
ifneq (${GO},)
	GO_BIN = ${GO}
endif

GO_SRC_PATH = $(GOPATH)/src
PB_PATH = $(shell pwd)/common/consensus
PB_PKG_PATH = ../consensus

help: Makefile
	@printf "${BLUE}Choose a command run:${NC}\n"
	@sed -n 's/^##//p' $< | column -t -s ':' | sed -e 's/^/    /'

## make prepare: Preparation before development
prepare:
	${GO_BIN} install go.uber.org/mock/mockgen@main
	${GO_BIN} install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.3
	${GO_BIN} install github.com/fsgo/go_fmt/cmd/gorgeous@latest
	${GO_BIN} install google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
	${GO_BIN} install github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto@main

## make generate: Generate code
generate:
	${GO_BIN} generate ./...

## make clean-pb: Clean protobuf file
clean-pb:
	rm -rf $(CURRENT_PATH)/*.pb.go

## make compile-pb: Compile protobuf file
compile-pb: clean-pb
	protoc --proto_path=$(PB_PATH) -I. -I$(GO_SRC_PATH) \
		--go_out=$(PB_PATH) \
		--plugin protoc-gen-go="$(GOPATH)/bin/protoc-gen-go" \
		--go-vtproto_out=$(PB_PATH) \
		--plugin protoc-gen-go-vtproto="$(GOPATH)/bin/protoc-gen-go-vtproto" \
		--go-vtproto_opt=features=marshal+marshal_strict+unmarshal+size+equal+clone \
		$(PB_PATH)/*.proto

## make fmt: Formats go source code
fmt:
	gorgeous -local github.com/axiomesh -mi

## make test: Run go unittest
test:
	${GO_BIN} generate ./...
	${GO_BIN} test -timeout 300s ./... -count=1

