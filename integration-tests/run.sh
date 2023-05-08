#!/bin/sh

dockerd -s vfs &

go test -v ./integration-tests/...
