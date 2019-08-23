#!/usr/bin/env bash
ORG_DIR="$GOPATH/src/github.com/ultramesh"
PROJECT_NAME="flato-rbft"
PROJECT_DIR="$ORG_DIR/$PROJECT_NAME/"

cd $PROJECT_DIR
git config --global url."git@git.hyperchain.cn:".insteadOf "https://git.hyperchain.cn/"
export GO111MODULE=on
export GOPROXY=https://goproxy.io
./scripts/stage-1.sh
go mod download
unset GOPROXY
./scripts/stage-2.sh
go mod download
