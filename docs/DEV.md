# Local development

## Cloud provisioning

Cloud hosts are provisioned through the use of an external library [cloud-provisioners](https://github.com/inlets/cloud-provision).

The host plan and Operating System for each cloud is set within the operator.

The workflow is:
* Detect a service requiring an IP address and create a Custom Resource for it
* Create host, get an ID back
* Store the ID in the Custom Resource
* Check the status of the host using the ID
* When the ID is active, update the IP address on the service and in the Custom Resource

When the operator, the Custom Resource or the Service is deleted, the cloud host will be deleted using its ID.

The provisioning library returns an ID that can be used to check the status of a cloud host, however some hosts cannot be identified by an ID alone. In these situations a composite key is built such as: `name|region|zone` or `name|projectid`, where as others simply take an id: `1234567`.

All providers need an access key to provision hosts, however there are several flags that you may need depending on which provider you are testing such as `--project-id`, `--zone` and `--secret-token`. 

See how to create service accounts and which parameters are required in the [inlets-operator reference docs](https://docs.inlets.dev/#/tools/inlets-operator?id=inlets-operator-reference-documentation)

## Run the Go binary with DigitalOcean

You can run the operator outside of the cluster as a local Go binary, this is the fastest way to test changes.

Assuming you're running a local cluster with [KinD](https://github.com/kubernetes-sigs/kind):

Sign up to [DigitalOcean.com](https://digitalocean.com) and get an access key, save it as a file with no extra whitespace or newlines to: `$HOME/do-access-token`.

Now fetch the code from GitHub:

```sh
export GOPATH=$HOME/go/
go get -u github.com/inlets/inlets-operator
cd $GOPATH/github.com/inlets/inlets-operator
```

Download dependencies and apply the required CRD for `tunnels.inlets.inlets.dev`

```sh
go get

kubectl apply -f artifacts/crds/

go build && ./inlets-operator  --kubeconfig $HOME/.kube/config \
  --access-key=$(cat ~/do-access-token) \
  --provider digitalocean \
  --region lon1
```

## 