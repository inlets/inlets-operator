# Local dev with Go binary

## Run the Go binary with Equinix Metal

Assuming you're running a local cluster with [KinD](https://github.com/kubernetes-sigs/kind):

Sign up to [Equnix Metal](https://equinix-metal.com) and get an access key, save it in `~/equinix-metal-token`

```sh
kubectl apply -f ./artifacts/crd.yaml

export EQUINIXMETAL_PROJECT_ID=""	# Populate from dashboard

export GOPATH=$HOME/go/
go get -u github.com/inlets/inlets-operator
cd $GOPATH/github.com/inlets/inlets-operator

go get

go build && ./inlets-operator  --kubeconfig "$(kind get kubeconfig-path --name="kind")" --access-key=$(cat ~/equinix-metal-token) --project-id="${EQUINIXMETAL_PROJECT_ID}"
```

## Run the Go binary with DigitalOcean

Assuming you're running a local cluster with [KinD](https://github.com/kubernetes-sigs/kind):

Sign up to [DigitalOcean.com](https://DigitalOcean.com) and get an access key, save it in `~/do-access-token`.

```sh
kubectl apply ./artifacts/crd.yaml

export GOPATH=$HOME/go/
go get -u github.com/inlets/inlets-operator
cd $GOPATH/github.com/inlets/inlets-operator

go get

go build && ./inlets-operator  --kubeconfig "$(kind get kubeconfig-path --name="kind")" --access-key=$(cat ~/do-access-token) --provider digitalocean
```

## Run the Go binary with Scaleway

Assuming you're running a local cluster with [KinD](https://github.com/kubernetes-sigs/kind):
Sign up to scaleway and get create your access and secret keys on [the credentials page](https://console.scaleway.com/account/credentials)

```sh
kubectl apply ./artifacts/crd.yaml

export GOPATH=$HOME/go/
go get -u github.com/inlets/inlets-operator
cd $GOPATH/github.com/inlets/inlets-operator

go get

go build && ./inlets-operator \
  --kubeconfig "$(kind get kubeconfig-path --name="kind")" \
  --provider=scaleway
  --access-key="ACCESS_KEY" --secret-key="SECRET_KEY" \
  --organization-id="ORG"
```
