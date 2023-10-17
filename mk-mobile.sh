#!/bin/bash

if [ -z "$GOPATH" ]; then
	echo no GOPATH
	exit 1
fi
export GOPRIVATE="github.com/nayutaco"

if [ $# -ne 1 ]; then
	echo './mk-mobile.sh [mobile / android / ios]'
	exit 1
fi
BUILD=$1

echo $PWD | grep 'src/github.com/lightningnetwork/lnd'
if [ $? != 0 ]; then
    echo FAIL: need clone \"src/github.com/lightningnetwork/lnd\"
    exit 1
fi

if [ $# == 1 ] && [ $1 == "clean" ]; then
    make clean clean-mobile
    exit 0
fi

go install golang.org/x/mobile/cmd/gomobile@latest
go mod download golang.org/x/mobile
gomobile init

go env -w GO111MODULE="off"
go get -u google.golang.org/grpc/test/bufconn
go get golang.org/x/mobile/bind
go env -w GO111MODULE="on"

echo
set -e
make $BUILD tags="signrpc walletrpc" prefix=1
if [ $BUILD == "android" ] || [ $BUILD == "mobile" ]; then
    ls -l mobile/build/android/Lndmobile.aar
fi
if [ $BUILD == "ios" ] || [ $BUILD == "mobile" ]; then
    ls -l mobile/build/ios/
fi
set +e
git status > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "not git directory"
    exit 0
fi
echo
git describe --tags --dirty
if ! git diff --quiet; then
    echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@"
    echo "@@@ This repository is DIRTY !!"
    echo "@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@"
fi
