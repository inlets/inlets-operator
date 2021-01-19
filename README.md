# inlets-operator

[![Build Status](https://travis-ci.com/inlets/inlets-operator.svg?branch=master)](https://travis-ci.com/inlets/inlets-operator) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT) [![Go Report Card](https://goreportcard.com/badge/github.com/inlets/inlets-operator)](https://goreportcard.com/report/github.com/inlets/inlets-operator) [![Documentation](https://godoc.org/github.com/inlets/inlets-operator?status.svg)](http://godoc.org/github.com/inlets/inlets-operator)

Add public LoadBalancers to your local Kubernetes clusters.

When using a managed [Kubernetes](https://kubernetes.io/) engine, you can expose a Service as a "LoadBalancer" and your cloud provider will provision a cloud load-balancer for you, and start routing traffic to the selected service inside your cluster. In other words, you get network ingress to an internal service.

The inlets-operator brings that same experience to your local Kubernetes cluster by provisioning an exit-server on the public cloud and running an inlets server process there.

Once the inlets-operator is installed, any Service of type LoadBalancer will get an IP address, unless you exclude it with an annotation.

```bash
kubectl expose deployment nginx-1 --port=80 --type=LoadBalancer

$ kubectl get services -w
NAME               TYPE        CLUSTER-IP        EXTERNAL-IP       PORT(S)   AGE
service/nginx-1    ClusterIP   192.168.226.216   <pending>         80/TCP    78s
service/nginx-1    ClusterIP   192.168.226.216   104.248.163.242   80/TCP    78s
```

## Who is this for?

Your cluster could be running anywhere: on your laptop, in an on-premises datacenter, within a VM, or on your Raspberry Pi. Ingress and LoadBalancers are a core-building block of Kubernetes clusters, so Ingress is especially important if you:

* run a private-cloud or a homelab
* self-host applications and APIs
* test and share work with colleagues or clients
* want to build a realistic environment
* integrate with webhooks and third-party APIs

There is no need to open a firewall port, set-up port-forwarding rules, configure dynamic DNS or any of the usual hacks. You will get a public IP and it will "just work" for any TCP traffic you may have.

## How is it better than other solutions?

* There are no rate limits for your services when exposed through a self-hosted inlets tunnel
* You can use your own DNS
* You can use your own IngressController
* You can take your IP address with you - wherever you go

Any Service of type `LoadBalancer` can be exposed within a few seconds.

Since exit-servers are created in your preferred cloud (around a dozen are supported already), you'll only have to pay for the cost of the VM, and where possible, the cheapest plan has already been selected for you. For example with Hetzner that's about 3 EUR / mo, and with DigitalOcean it comes in at around 5 USD - both of these VPSes come with generous bandwidth allowances, global regions and fast network access.

## Video demo

Watch a video walk-through where we deploy an IngressController (ingress-nginx) to KinD, and then obtain LetsEncrypt certificates using cert-manager.

![Video demo](https://img.youtube.com/vi/4wFSdNW-p4Q/hqdefault.jpg)

[Try the step-by-step tutorial](https://docs.inlets.dev/#/get-started/quickstart-ingresscontroller-cert-manager?id=quick-start-expose-your-ingresscontroller-and-get-tls-from-letsencrypt-and-cert-manager)

## inlets tunnel capabilities

The operator detects Services of type LoadBalancer, and then creates a `Tunnel` Custom Resource. Its next step is to provision a small VM with a public IP on the public cloud, where it will run the inlets tunnel server. Then an inlets client is deployed as a Pod within your local cluster, which connects to the server and acts like a gateway to your chosen local service.

### Powered by [inlets PRO](https://github.com/inlets/inlets-pro)

* Automatic end-to-end encryption of the control-plane using PKI and TLS
* Punch out multiple ports such as 80 and 443 over the same tunnel
* Tunnel any TCP traffic at L4 i.e. Mongo, Postgres, MariaDB, Redis, NATS, SSH and TLS itself.
* Tunnel an IngressController including TLS termination and LetsEncrypt certs from cert-manager
* Commercially licensed and supported. For cloud native operators and developers.

Heavily discounted [pricing available](https://inlets.dev/) for personal use.

## Status and backlog

Operator cloud host provisioning:

- [x] Provision VMs/exit-nodes on public cloud: [Equinix-Metal](https://equinix-metal.com), DigitalOcean, Scaleway, GCP, AWS EC2, Linode and Azure

With [`inlets-pro`](https://github.com/inlets/inlets-pro) configured, you get the following additional benefits:

- [x] Automatic configuration of TLS and encryption using secured websocket `wss://` for control-port
- [x] Tunnel pure TCP traffic
- [x] Separate data-plane (ports given by Kubernetes) and control-plane (port `8132`)

Other features:

- [x] Automatically update Service type LoadBalancer with a public IP
- [x] Tunnel L4 `tcp` traffic
- [x] In-cluster Role, Dockerfile and YAML files
- [x] Raspberry Pi / armhf build and YAML file
- [x] ARM64 (Graviton/Odroid/Equinix-Metal) Dockerfile/build and K8s YAML files
- [x] Control which services get a LoadBalancer using annotations
- [x] Garbage collect hosts when Service or CRD is deleted
- [x] CI with Travis and automated release artifacts
- [x] One-line installer [arkade](https://get-arkade.dev/) - `arkade install inlets-operator --help`

## inlets-operator reference documentation for different cloud providers

Check out the reference documentation for inlets-operator to get exit-nodes provisioned on different cloud providers [here](https://docs.inlets.dev/#/tools/inlets-operator?id=inlets-operator-reference-documentation).

## Get an IP address for your IngressController and LetsEncrypt certificates

Unlike other solutions, this:

* Integrates directly into Kubernetes
* Gives you a TCP LoadBalancer, and updates its IP in `kubectl get svc`
* Allows you to use any custom DNS you want
* Works with LetsEncrypt

Example tutorials:

* [Setup Ingress, LetsEncrypt and a generic Node.js microservice](https://docs.inlets.dev/#/get-started/quickstart-ingresscontroller-cert-manager?id=expose-your-ingresscontroller-and-get-tls-from-letsencrypt)
* [Setup Ingress, LetsEncrypt and OpenFaaS](https://inlets.dev/blog/2020/10/15/openfaas-public-endpoints.html)
* [Setup Ingress, LetsEncrypt and a Docker Registry](https://blog.alexellis.io/get-a-tls-enabled-docker-registry-in-5-minutes/)

## Expose a service with a LoadBalancer

The LoadBalancer type is usually provided by a cloud controller, but when that is not available, then you can use the inlets-operator to get a public IP and ingress.

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

## Annotations, ignoring services and running with other LoadBalancers controllers

By default the operator will create a tunnel for every LoadBalancer service.

There are three ways to override the behaviour:

### 1) Create LoadBalancers for every service, unless annotated

To ignore a service such as `traefik` type in: `kubectl annotate svc/traefik -n kube-system dev.inlets.manage=false`

### 2) Create LoadBalancers for only annotated services

You can also set the operator to ignore the services by default and only manage them when the annotation is true with the flag `-annotated-only`
To create a service such as `traefik` type in: `kubectl annotate svc/traefik -n kube-system dev.inlets.manage=true`

### 3) Create a Tunnel resource for ClusterIP services

Running multiple LoadBalancers controllers together, e.g. inlets-operator and MetalLB, can have some issue as both will compete against each other when processing the service.

Although the inlets-operator has the flag `-annotated-only` to filter the services, not all other LoadBalancer controller have a similar feature.

In this case, the inlets-operator is still able to expose services by using a ClusterIP service with a Tunnel resource instead of a LoadBalancer service.

Example:

```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: nginx
spec:
  type: ClusterIP
  ports:
  - name: http
    port: 80
    targetPort: 80
  selector:
    app: nginx
---
apiVersion: inlets.inlets.dev/v1alpha1
kind: Tunnel
metadata:
  name: nginx
spec:
  serviceName: nginx
  auth_token: <token>
```

The public IP address of the tunnel is available in the service resource:

```bash
$ kubectl get services,tunnel
NAME            TYPE        CLUSTER-IP        EXTERNAL-IP       PORT(S)   AGE
service/nginx   ClusterIP   192.168.226.216   104.248.163.242   80/TCP    78s

NAME                             SERVICE   TUNNEL         HOSTSTATUS   HOSTIP            HOSTID
tunnel.inlets.inlets.dev/nginx   nginx     nginx-client   active       104.248.163.242   214795742
```

or use a jsonpath to get the value:

```bash
kubectl get service nginx --output jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

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

The host [provisioning code](https://github.com/inlets/cloud-provision) used by the inlets-operator is shared with [inletsctl](https://github.com/inlets/inletsctl), both tools use the configuration in the grid below.

These costs need to be treated as an estimate and will depend on your bandwidth usage and how many hosts you decide to create. You can at all times check your cloud provider's dashboard, API, or CLI to view your exit-nodes. The hosts provided have been chosen because they are the absolute lowest-cost option that the maintainers could find.

| Provider                                                           | Price per month | Price per hour |     OS image | CPU | Memory | Boot time |
| ------------------------------------------------------------------ | --------------: | -------------: | -----------: | --: | -----: | --------: |
| [Google Compute Engine](https://cloud.google.com/compute)                                          |         *  ~\$4.28 |       ~\$0.006 | Debian GNU Linux 9 (stretch) | 1 | 614MB | ~3-15s |
| [Equinix-Metal](https://equinix-metal.com)           |           ~\$51 |         \$0.07 | Ubuntu 16.04 |   4 |    8GB | ~45-60s  |
| [Digital Ocean](https://www.digitalocean.com/pricing/#Compute)     |             \$5 |      ~\$0.0068 | Ubuntu 16.04 |   1 |  1GB | ~20-30s  |
| [Scaleway](https://www.scaleway.com/en/pricing/#virtual-instances) |           2.99€ |         0.006€ | Ubuntu 18.04 |   2 |    2GB | 3-5m      |

* The first f1-micro instance in a GCP Project (the default instance type for inlets-operator) is free for 720hrs(30 days) a month 

## Contributing

Contributions are welcome, see the [CONTRIBUTING.md](CONTRIBUTING.md) guide.

## Similar projects / products and alternatives

- [inlets pro](https://github.com/inlets/inlets-pro) - L4 TCP tunnel, which can tunnel any TCP traffic with automatic, built-in encryption. Kubernetes-ready with Docker images and YAML manifests. 
- [inlets](https://inlets.dev) - inlets provides an L7 HTTP tunnel for applications through the use of an exit node, it is used by the inlets operator. Encryption must be configured separately.
- [metallb](https://github.com/danderson/metallb) - open source LoadBalancer for private Kubernetes clusters, no tunnelling.
- [Cloudflare Argo](https://www.cloudflare.com/en-gb/products/argo-tunnel/) - paid SaaS product from Cloudflare for Cloudflare customers and domains - K8s integration available through Ingress
- [ngrok](https://ngrok.com) - a popular tunnelling tool, restarts every 7 hours, limits connections per minute, paid SaaS product with no K8s integration available

## Author / vendor

inlets and the inlets-operator are brought to you by [OpenFaaS Ltd](https://www.openfaas.com) and [Alex Ellis](https://www.alexellis.io/).
