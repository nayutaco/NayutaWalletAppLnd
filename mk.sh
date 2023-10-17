#!/bin/bash

if [ -z "$GOPATH" ]; then
	echo no GOPATH
	exit 1
fi
TAGS="signrpc walletrpc"
make tags="$TAGS" && make build tags="$TAGS"
cp lnd-debug $GOPATH/bin/lnd-core
cp lncli-debug $GOPATH/bin/lncli-core
