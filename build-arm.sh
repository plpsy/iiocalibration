#!/bin/bash

# Architecture	Status	GOARM value	GOARCH value
# ARMv4 and below	not supported	n/a	n/a
# ARMv5	supported	GOARM=5	GOARCH=arm
# ARMv6	supported	GOARM=6	GOARCH=arm
# ARMv7	supported	GOARM=7	GOARCH=arm
# ARMv8	supported	n/a	GOARCH=arm64


export GOOS=linux
export GOARCH=${1:-arm}
export GO111MODULE="off"
export GOARM=${2:-7}
echo GOARCH=$GOARCH, GOARM=$GOARM



VERSION=$(git describe --abbrev=0 --tags)
GIT_COMMIT=$(git rev-parse --short HEAD)
VERPKG="github.com/plpsy/iiocalibration/version"
LDFLAGS="-X '$VERPKG.version=$(git describe --abbrev=0 --tags)' -X '$VERPKG.gitCommit=$(git rev-parse --short HEAD)' -X '$VERPKG.buildAt=$(date)' -w -extldflags '-static -s'"

go build -ldflags="$LDFLAGS" 



