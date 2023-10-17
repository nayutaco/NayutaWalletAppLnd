#!/bin/bash
# # protoc v3.19.1
# PB_REL="https://github.com/protocolbuffers/protobuf/releases"
# curl -LO $PB_REL/download/v3.19.1/protoc-3.19.1-linux-x86_64.zip
# unzip protoc-3.19.1-linux-x86_64.zip -d $HOME/.local
# export PATH="$PATH:$HOME/.local/bin"
#
# go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.27.1
# go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

# https://grpc.io/docs/languages/go/quickstart/#regenerate-grpc-code
protoc --go_out=.. --go_opt=paths=source_relative --go-grpc_out=.. --go-grpc_opt=paths=source_relative lspclient.proto
