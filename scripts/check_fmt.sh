#!/bin/bash

if [ "$(gofmt -l . | wc -l)" -gt 0 ]; then
    echo "Unformatted files found:"
    # Run again to print files. Couldn't figure out a way to
    # save the output and make it work with wc.
    gofmt -l .
    exit 1
fi
