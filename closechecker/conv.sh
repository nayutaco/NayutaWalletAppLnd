#!/bin/bash

ELECTRUM_SERVER_FILE="servers-018a83078c93cfb5c14507d1bc62bd5baa2af825.json"
GO_FILE="servers-array.txt"
cat ${ELECTRUM_SERVER_FILE} | \
        sed -e 's/\(\".*\"\): {/{"server": \1,/g' -e 's/^{/[/g' -e 's/^}/]/g' | \
        jq '[.[] | select(.server | endswith(".onion") == false)]' | \
        jq '[.[] | select(.server | endswith(".0") == false)]' | \
        sed -e 's/"server":/Server:/g' \
                -e 's/"s":/TLS:/g' \
                -e 's/"t":/TCP:/g' \
                -e 's/"pruning":.*$//g' \
                -e 's/"version":.*$//g' | \
        sed 's/^[[:blank:]]*$//' | \
        sed '/^$/d' \
> ${GO_FILE}
