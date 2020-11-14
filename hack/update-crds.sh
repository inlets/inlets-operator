#!/bin/bash

export controllergen="$GOPATH/bin/controller-gen"
export PKG=sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.0

if [ ! -e "$controllergen" ]
then
echo "Getting $PKG"
    go install $PKG
fi

echo $controllergen

"$controllergen" \
  crd \
  schemapatch:manifests=./artifacts/crds \
  paths=./pkg/apis/... \
  output:dir=./artifacts/crds

# Some versions of controller-tools generate storedVersions and conditions as null,
# We need to change them to []
sed -i.bak \
  -e 's/conditions: null/conditions: \[\]/' \
  -e 's/storedVersions: null/storedVersions: \[\]/' \
  ./artifacts/crds/*.yaml
rm -f ./artifacts/crds/*.bak
