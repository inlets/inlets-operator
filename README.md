# inlets-operator

[![Build Status](https://travis-ci.com/inlets/inlets-operator.svg?branch=master)](https://travis-ci.com/inlets/inlets-operator) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT) [![Go Report Card](https://goreportcard.com/badge/github.com/inlets/inlets-operator)](https://goreportcard.com/report/github.com/inlets/inlets-operator) [![Documentation](https://godoc.org/github.com/inlets/inlets-operator?status.svg)](http://godoc.org/github.com/inlets/inlets-operator)

"Get a Kubernetes LoadBalancer where you never thought it was possible."

In cloud-based [Kubernetes](https://kubernetes.io/) solutions, Services can be exposed as type "LoadBalancer" and your cloud provider will provision a LoadBalancer and start routing traffic, in another word: you get ingress to your service.

inlets-operator brings that same experience to your local Kubernetes or k3s cluster (k3s/k3d/minikube/microk8s/Docker Desktop/KinD). The operator automates the creation of an [inlets](https://inlets.dev) exit-node on public cloud, and runs the client as a Pod inside your cluster. Your Kubernetes `Service` will be updated with the public IP of the exit-node and you can start receiving incoming traffic immediately.

## Who is this for?

This solution is for users who want to gain incoming network access (ingress) to their private Kubernetes clusters running on their laptops, VMs, within a Docker container, on-premises, or behind NAT. The cost of the LoadBalancer with a IaaS like DigitalOcean is around 5 USD / mo, which is 10 USD cheaper than an AWS ELB or GCP LoadBalancer.

Whilst 5 USD is cheaper than a "Cloud Load Balancer", this tool is for users who cannot get incoming connections due to their network configuration, not for saving money vs. public cloud.

You can configure the operator to use either of our tunnels: inlets OSS for L7 HTTP traffic, or inlets PRO which adds L4 TCP support, automatic encryption with TLS and can enable the use of an IngressController and cert-manager directly from your laptop or private cloud.

## inlets tunnel capabilities

The operator detects Services of type LoadBalancer, and then creates a `Tunnel` Custom Resource. Its next step is to provision a small VM with a public IP on the public cloud, where it will run the inlets tunnel server. Then an inlets client is deployed as a Pod within your local cluster, which connects to the server and acts like a gateway to your chosen local service.

Pick your inlets edition:

### [inlets PRO](https://github.com/inlets/inlets-pro)

* Tunnel an IngressController including TLS termination and LetsEncrypt certs from cert-manager
* Tunnel any TCP traffic at L4 i.e. Mongo, Postgres, MariaDB, Redis, NATS, SSH and TLS itself.
* Automatic end-to-end encryption built-in with TLS.
* Commercially licensed and supported. For cloud native operators and developers.
* Punch out multiple ports such as 80 and 443 over the same tunnel

Discounted [pricing available](https://docs.inlets.dev/#/pricing/?id=pricing) for personal use.

### [inlets OSS](https://github.com/inlets/inlets)

* Tunnel L7 HTTP traffic.
* Free, OSS, built for community developers.
* Punch out only one port per tunnel, port name must be: `http`
* No encryption enabled by default.

## Status and backlog

Operator cloud host provisioning:

- [x] Provision VMs/exit-nodes on public cloud
  - [x] Provision to [Packet.com](https://packet.com)
  - [x] Provision to DigitalOcean
  - [x] Provision to Scaleway
  - [x] Provision to GCP
  - [x] Provision to AWS EC2
  - [x] Provision to Linode
  - [x] Provision to Azure
- [x] Publish stand-alone [Go provisioning library/SDK](https://github.com/inlets/inletsctl/tree/master/pkg/provision)

With [`inlets-pro`](https://github.com/inlets/inlets-pro) configured, you get the following additional benefits:

- [x] Automatic configuration of TLS and encryption using secured websocket `wss://` for control-port
- [x] Tunnel pure TCP traffic
- [x] Separate data-plane (ports given by Kubernetes) and control-plane (port `8132`)

Other features:

- [x] Automatically update Service type LoadBalancer with a public IP
- [x] Tunnel L7 `http` traffic
- [x] In-cluster Role, Dockerfile and YAML files
- [x] Raspberry Pi / armhf build and YAML file
- [x] ARM64 (Graviton/Odroid/Packet.com) Dockerfile/build and K8s YAML files
- [x] Ignore Services with `dev.inlets.manage: false` annotation
- [x] Garbage collect hosts when Service or CRD is deleted
- [x] CI with Travis and automated release artifacts
- [x] One-line installer [arkade](https://get-arkade.dev/) - `arkade install inlets-operator --help`

Backlog pending:
- [ ] Provision to Civo

### inlets projects

Inlets is a Cloud Native Tunnel and is [listed on the Cloud Native Landscape](https://landscape.cncf.io/category=service-proxy&format=card-mode&grouping=category&sort=stars) under *Service Proxies*.

* [inlets](https://github.com/inlets/inlets) - Cloud Native Tunnel for L7 / HTTP traffic written in Go
* [inlets-pro](https://github.com/inlets/inlets-pro-pkg) - Cloud Native Tunnel for L4 TCP
* [inlets-operator](https://github.com/inlets/inlets-operator) - Public IPs for your private Kubernetes Services and CRD
* [inletsctl](https://github.com/inlets/inletsctl) - Automate the cloud for fast HTTP (L7) and TCP (L4) tunnels

## Author

inlets and inlets-operator are brought to you by [Alex Ellis](https://twitter.com/alexellisuk). Alex is a [CNCF Ambassador](https://www.cncf.io/people/ambassadors/) and the founder of [OpenFaaS](https://github.com/openfaas/faas/).

`inlets` is made available free-of-charge, but you can support its ongoing development through [GitHub Sponsors](https://insiders.openfaas.io/) 💪

## Video demo

This video demo shows a single-node VM running on k3s on Packet.com, and the inlets exit node also being provisioned on Packet's infrastructure.

[![https://img.youtube.com/vi/LeKMSG7QFSk/0.jpg](https://img.youtube.com/vi/LeKMSG7QFSk/0.jpg)](https://www.youtube.com/watch?v=LeKMSG7QFSk&amp=&feature=youtu.be)

See an alternative video showing my cluster running with KinD on my Mac and the exit node being provisioned on DigitalOcean:

- [KinD & DigitalOcean](https://youtu.be/c6DTrNk9zRk).

## Step-by-step tutorial

[Try the step-by-step tutorial](https://blog.alexellis.io/ingress-for-your-local-kubernetes-cluster/)

## Running in-cluster, using DigitalOcean for the exit node

> Note: this example is now multi-arch, so it's valid for `x86_64`, `ARMHF`, and `ARM64`.

You can also run the operator in-cluster, a ClusterRole is used since Services can be created in any namespace, and may need a tunnel. Install the inlets-operator using a single command with [arkade](https://get-arkade.dev/). arkade runs against any Kubernetes cluster and wraps the helm command-line.

Install with inlets PRO:

```bash
arkade install inlets-operator \
 --provider digitalocean \
 --region lon1 \
 --token-file $HOME/Downloads/do-access-token \
 --license $(cat $HOME/inlets-pro-license.txt)
```

Install with inlets OSS:

```bash
arkade install inlets-operator \
 --provider digitalocean \
 --region lon1 \
 --token-file $HOME/Downloads/do-access-token
```

## Using a provider which requires an Access Key and Secret Key? (AWS EC2, Scaleway)

These providers require an additional secret to be provided, this can be created like this:
 
To install using `arkade` you can pass the additional --secret-key-file arg

```bash
arkade install inlets-operator \
 --provider ec2 \
 --region eu-west-1 \
 --token-file $HOME/Downloads/access-key \
 --secret-key-file $HOME/Downloads/secret-access-key \
 --license $(cat $HOME/inlets-pro-license.txt)
```
 
## Running in-cluster, using Google Compute Engine for the exit node using helm

> Note: this example is now multi-arch, so it's valid for `x86_64`, `ARMHF`, and `ARM64`.

If you do not have helm installed and configured follow the instructions [here](https://github.com/openfaas/faas-netes/blob/master/HELM.md)

It is assumed that you have gcloud installed and configured on your machine.
If not, then follow the instructions [here](https://cloud.google.com/sdk/docs/quickstarts)

```bash
# Get current projectID
export PROJECTID=$(gcloud config get-value core/project 2>/dev/null)

# Create a service account
gcloud iam service-accounts create inlets \
--description "inlets-operator service account" \
--display-name "inlets"

# Get service account email
export SERVICEACCOUNT=$(gcloud iam service-accounts list | grep inlets | awk '{print $2}')

# Assign appropriate roles to inlets service account
gcloud projects add-iam-policy-binding $PROJECTID \
--member serviceAccount:$SERVICEACCOUNT \
--role roles/compute.admin

gcloud projects add-iam-policy-binding $PROJECTID \
--member serviceAccount:$SERVICEACCOUNT \
--role roles/iam.serviceAccountUser

# Create inlets service account key file
gcloud iam service-accounts keys create key.json \
--iam-account $SERVICEACCOUNT

# Create a secret to store the service account key file
kubectl create secret generic inlets-access-key --from-file=inlets-access-key=key.json

# Add and update the inlets-operator helm repo
helm repo add inlets https://inlets.github.io/inlets-operator/

helm repo update

# Install inlets-operator with the required fields
helm upgrade inlets-operator --install inlets/inlets-operator \
  --set provider=gce,zone=us-central1-a,projectID=$PROJECTID
```

## Running in-cluster, using Linode for the exit node

Install using helm:

```bash
kubectl apply -f ./artifacts/crds/

# Create a secret to store the service account key file
kubectl create secret generic inlets-access-key --from-literal inlets-access-key=<Linode API Access Key>

# Add and update the inlets-operator helm repo
helm repo add inlets https://inlets.github.io/inlets-operator/

helm repo update

# Install inlets-operator with the required fields
helm upgrade inlets-operator --install inlets/inlets-operator \
  --set provider=linode,region=us-east
```

You can also install the inlets-operator using a single command using [arkade](https://get-arkade.dev/), arkade runs against any Kubernetes cluster.

Install with inlets PRO:

```bash
arkade install inlets-operator \
 --provider linode \
 --region us-east \
 --access-key <Linode API Access Key> \
 --license $(cat $HOME/inlets-pro-license.txt)
```

Install with inlets OSS:

```bash
arkade install inlets-operator \
 --provider linode \
 --region us-east \
 --access-key <Linode API Access Key>
```

## Running in-cluster, using Azure for the exit node
Prerequisites:

* You will need `az`. See [Install the Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)

Generate Azure auth file 
```sh
az ad sp create-for-rbac --sdk-auth > ~/Downloads/client_credentials.json
```

Install using helm:
```bash
kubectl apply -f ./artifacts/crds/
kubectl create secret generic inlets-access-key --from-file=inlets-access-key=~/Downloads/client_credentials.json
helm repo add inlets https://inlets.github.io/inlets-operator/
helm repo update
helm upgrade inlets-operator --install inlets/inlets-operator \
  --set provider=azure,region=eastus,subscriptionID=<Azure Subscription ID> 
```

## Expose a service with a LoadBalancer

The LoadBalancer type is usually provided by a cloud controller, but when that is not available, then you can use the inlets-operator to get a public IP and ingress. The free OSS version of inlets provides a HTTP tunnel, inlets PRO can provide TCP and full functionality to an IngressController.

First create a deployment for Nginx.

For Kubernetes 1.17 and lower:

```bash
kubectl run nginx-1 --image=nginx --port=80 --restart=Always
```

For 1.18 and higher:

```bash
kubectl apply -f https://raw.githubusercontent.com/inlets/inlets-operator/master/contrib/nginx-sample-deployment.yaml
```

Now create a service of type LoadBalancer via `kubectl expose`:

```bash
kubectl expose deployment nginx-1 --port=80 --type=LoadBalancer
kubectl get svc

kubectl get tunnel/nginx-1-tunnel -o yaml

kubectl logs deploy/nginx-1-tunnel-client
```

Check the IP of the LoadBalancer and then access it via the Internet.

## Get an IngressController with TLS certificates

You can bring your own IngressController such as ingress-nginx or Traefik. And if you are using inlets PRO, you can also get TLS termination and certificates from LetsEncrypt via cert-manager.

* [Expose Your IngressController and get TLS from LetsEncrypt](https://docs.inlets.dev/#/get-started/quickstart-ingresscontroller-cert-manager?id=expose-your-ingresscontroller-and-get-tls-from-letsencrypt)

## Notes on OSS inlets

inlets PRO can tunnel multiple ports, but inlets OSS is set to take the first port named "http" for your service. With the OSS version of inlets (see example with OpenFaaS), make sure you give the `port` a `name` of `http`, otherwise a default of `80` will be used incorrectly.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: gateway
  namespace: openfaas
  labels:
    app: gateway
spec:
  ports:
    - name: http
      port: 8080
      protocol: TCP
      targetPort: 8080
      nodePort: 31112
  selector:
    app: gateway
  type: LoadBalancer
```

## Annotations and ignoring services

By default the operator will create a tunnel for every LoadBalancer service.

To ignore a service such as `traefik` type in: `kubectl annotate svc/traefik -n kube-system dev.inlets.manage=false`

You can also set the operator to ignore the services by default and only manage them when the annotation is true. `dev.inlets.manage=true`
To do this, run the operator with the flag `-annotated-only`

## Monitor/view logs

The operator deployment is in the `kube-system` namespace.

```sh
kubectl logs deploy/inlets-operator -n kube-system -f
```

## Running on a Raspberry Pi

Use the same commands as described in the section above.

> There used to be separate deployment files in `artifacts` folder called `operator-amd64.yaml` and `operator-armhf.yaml`.
> Since version `0.2.7` Docker images get built for multiple architectures with the same tag which means that there is now just one deployment file called `operator.yaml` that can be used on all supported architecures.

# Provider Pricing

The host [provisioning code](https://github.com/inlets/inletsctl/tree/master/pkg/provision) used by the inlets-operator is shared with [inletsctl](https://github.com/inlets/inletsctl), both tools use the configuration in the grid below.

These costs need to be treated as an estimate and will depend on your bandwidth usage and how many hosts you decide to create. You can at all times check your cloud provider's dashboard, API, or CLI to view your exit-nodes. The hosts provided have been chosen because they are the absolute lowest-cost option that the maintainers could find.

| Provider                                                           | Price per month | Price per hour |     OS image | CPU | Memory | Boot time |
| ------------------------------------------------------------------ | --------------: | -------------: | -----------: | --: | -----: | --------: |
| [Google Compute Engine](https://cloud.google.com/compute)                                          |         *  ~\$4.28 |       ~\$0.006 | Debian GNU Linux 9 (stretch) | 1 | 614MB | ~3-15s |
| [Packet](https://www.packet.com/cloud/servers/t1-small/)           |           ~\$51 |         \$0.07 | Ubuntu 16.04 |   4 |    8GB | ~45-60s  |
| [Digital Ocean](https://www.digitalocean.com/pricing/#Compute)     |             \$5 |      ~\$0.0068 | Ubuntu 16.04 |   1 |  512MB | ~20-30s  |
| [Scaleway](https://www.scaleway.com/en/pricing/#virtual-instances) |           2.99€ |         0.006€ | Ubuntu 18.04 |   2 |    2GB | 3-5m      |

* The first f1-micro instance in a GCP Project (the default instance type for inlets-operator) is free for 720hrs(30 days) a month 

You can [purchase inlets PRO here](https://docs.inlets.dev/#/pricing/)

## Contributing

Contributions are welcome, see the [CONTRIBUTING.md](CONTRIBUTING.md) guide.

## Similar projects / products and alternatives

- [inlets pro](https://github.com/inlets/inlets-pro) - L4 TCP tunnel, which can tunnel any TCP traffic with automatic, built-in encryption. Kubernetes-ready with Docker images and YAML manifests. 
- [inlets](https://inlets.dev) - inlets provides an L7 HTTP tunnel for applications through the use of an exit node, it is used by the inlets operator. Encryption can be configured separately.
- [metallb](https://github.com/danderson/metallb) - open source LoadBalancer for private Kubernetes clusters, no tunnelling.
- [Cloudflare Argo](https://www.cloudflare.com/en-gb/products/argo-tunnel/) - paid SaaS product from Cloudflare for Cloudflare customers and domains - K8s integration available through Ingress
- [ngrok](https://ngrok.com) - a popular tunnelling tool, restarts every 7 hours, limits connections per minute, paid SaaS product with no K8s integration available
