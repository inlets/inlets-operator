#!/bin/bash

export controllergen="$GOPATH/bin/controller-gen"
export PKG=sigs.k8s.io/controller-tools/cmd/controller-gen

if [ ! -e "$gen" ]
then
echo "Getting $PKG"
    GO111MODULE=off go get $PKG
fi

echo $controllergen

echo "$controllergen" \
  crd \
  schemapatch:manifests=./artifacts/crds \
  paths=./pkg/apis/... \
  output:dir=./artifacts/crds
