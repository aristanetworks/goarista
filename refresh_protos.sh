#!/usr/bin/env bash
# Copyright (c) 2022 Arista Networks, Inc.
# Use of this source code is governed by the Apache License 2.0
# that can be found in the COPYING file.

# This script is run periodically by Jenkins in the update-deps job
# It is used to ensure that .pb.go files are generated with the most
# up-to-date protoc compiler


if [[ "$USER" != jenkins ]]; then
  echo >&2 "warning: This script is mostly for Jenkins' benefit."
  echo >&2 "         It updates dependencies and posts a review to Gerrit."
  echo >&2 "         Hit enter to proceed anyway, Ctrl-C to cancel."
  test -t 0 && read -rs # Read enter if stdin is a terminal
fi

set -euo pipefail
cd "$(dirname "$0")"

# find the latest release of protoc compiler, download and unzip
curl -s https://api.github.com/repos/protocolbuffers/protobuf/releases/latest \
 | grep -o -m 1 \
"https://github.com/protocolbuffers/protobuf/releases/download/v.*/protoc\-.*-linux-x86_64.zip" \
 | xargs curl -LO && echo "protoc downloaded"
python3 -c \
"from glob import glob as g;from zipfile import ZipFile as z;
p=g('protoc-*-linux-x86_64.zip')[0];f=z(p,'r');f.extractall('protoc');f.close()"
rm protoc-*-linux-x86_64.zip
protocBin=$(realpath .)"/protoc/bin/protoc"
chmod +x "$protocBin"

# install the latest protoc-gen-go and add to path
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
protoc-gen-go --version
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
protoc-gen-go-grpc --version

protocBin=$(realpath ./protoc/bin/protoc);
startPath=$(realpath .);

# Copy dependencies into a vendor folder
go mod vendor
# Add the two places where we could import proto files from: Anywhere under the GOPATH/src,
# and anything under the vendor directory.
# We can't use the GOPATH/pkg/mod directory as its package names are not in the format that
# protoc is expecting.
# E.g. protoc is expecting the github.com/openconfig/gnmi module to exist at a directory like:
# github.com/openconfig/gnmi
# When it really exists at a transient directory like:
# github.com/openconfig/gnmi@v0.0.0-20220503232738-6eb133c65a13
# By vendoring the dependencies we create a copy of them which has the expected naming
protoImports="${GOPATH}/src:${startPath}/vendor";

# filePaths is a space-seperated string of all .proto files in goarista
# exclude ./.git, ./modLink/ and the downloaded protoc directory
filePaths=$(find . -not \( -path ./.git -prune \) -not \( -path ./protoc -prune \) \
  -not \( -path ./vendor -prune \) -type f -name "*.proto") ;

# This loop recompiles every .proto file under goarista, and if
# significant changes in any of the generated files are observed, it will
# add the updated .pb.go and _grpc.pb.go files to git
for filePath in $filePaths; do
      echo "filePath is $filePath"

      protoFile="${filePath##*/}";
      pkgPath="${filePath%/*}";
      cd "${pkgPath}";
      pkg="${pkgPath##*/}";
      $protocBin -I="${protoImports}" \
          --proto_path=. \
          --go_out=. \
          --go_opt=paths=source_relative   \
          --go_opt="M${protoFile}=./${pkg}"  \
          --go-grpc_out=. \
          --go-grpc_opt=paths=source_relative \
          --go-grpc_opt="M${protoFile}=./${pkg}"\
           "${protoFile}";

      echo "proto compiled successfully"

      # determine if there are any significant differences in the newly generated files.
      # diffs which _only_ involve updates to the version numbers are deemed not-significant.
      # If the diff is significant, diffExists = 1, else diffExists = 0
      read -r diffExists < <(git --no-pager diff --no-color --unified=0 ./ | grep -Ev \
      -e "^(\+|-)//\s+(-|)\s*protoc(|-gen-go)\s+v[0-9\.]" \
      -e "^(@@|diff|index|--- a/|\+\+\+ b/)" | head -c1 | wc -c)

      echo "diffExists is $diffExists"

      git add ./\*.pb.go;
      if [ "$diffExists" -eq "0"  ];then
        echo "restoring .pb.go files - no significant diff after re-compilation"
        git reset HEAD ./\*.pb.go;
        git checkout -- ./\*.pb.go;
      fi;

      cd "${startPath}";
      git status -uno;
done

rm -rf protoc/
rm -rf vendor/