#!/bin/bash

set -eo pipefail

shed() {
    go run main.go "$@"
}

if [ "$(shed run goimports -- -l . | wc -l)" -gt 0 ]; then
    echo "Unformatted files found:"
    # Run again to print files. Couldn't figure out a way to
    # save the output and make it work with wc.
    shed run goimports -- -l .
    exit 1
fi
