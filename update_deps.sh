#!/usr/bin/env bash
# Copyright (c) 2022 Arista Networks, Inc.
# Use of this source code is governed by the Apache License 2.0
# that can be found in the COPYING file.

# This script is run periodically by Jenkins to post a review to
# update all of our dependencies and proto files.

set -eax

if [[ "$USER" != jenkins ]]; then
  echo >&2 "warning: This script is mostly for Jenkins' benefit."
  echo >&2 "         It updates dependencies and posts a review to Gerrit."
  echo >&2 "         Hit enter to proceed anyway, Ctrl-C to cancel."
  test -t 0 && read -rs # Read enter if stdin is a terminal
fi

set -xe
if ! test -x .git/hooks/commit-msg; then
  scp -o BatchMode=yes -p -P 29418 gerrit.corp.arista.io:hooks/commit-msg .git/hooks/
fi

cd "$(dirname "$0")"
go get -u ./... && go mod tidy -go=1.16 && go mod tidy -go=1.17
git add go.mod go.sum

./refresh_protos.sh

git commit -m "update dependencies and protos

Jenkins-Job-Name: $JOB_NAME
Jenkins-Build-Number: $BUILD_NUMBER"
