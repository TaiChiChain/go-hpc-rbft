SHELL := /bin/bash

GO_BIN = go
ifneq (${GO},)
	GO_BIN = ${GO}
endif

help: Makefile
	@printf "${BLUE}Choose a command run:${NC}\n"
	@sed -n 's/^##//p' $< | column -t -s ':' | sed -e 's/^/    /'

## make prepare: Preparation before development
prepare:
	${GO_BIN} install github.com/golang/mock/mockgen@v1.7.0-rc.1
	${GO_BIN} install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.3
	${GO_BIN} install github.com/fsgo/go_fmt@v0.5.0

## make generate: Generate code
generate:
	${GO_BIN} generate ./...

## make compile-pb: Compile protobuf file
compile-pb:
	cd common/consensus && make pb

## make fmt: Formats go source code
fmt:
	go_fmt -local github.com/axiomesh -mi

## make test: Run go unittest
test:
	${GO_BIN} generate ./...
	${GO_BIN} test -timeout 300s ./... -count=1