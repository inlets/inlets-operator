# inlets-operator

[![build](https://github.com/inlets/inlets-operator/actions/workflows/build.yaml/badge.svg)](https://github.com/inlets/inlets-operator/actions/workflows/build.yaml) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT) [![Go Report Card](https://goreportcard.com/badge/github.com/inlets/inlets-operator)](https://goreportcard.com/report/github.com/inlets/inlets-operator) [![Documentation](https://godoc.org/github.com/inlets/inlets-operator?status.svg)](http://godoc.org/github.com/inlets/inlets-operator)

Get public TCP LoadBalancers for local Kubernetes clusters

When using a managed [Kubernetes](https://kubernetes.io/) engine, you can expose a Service as a "LoadBalancer" and your cloud provider will provision a TCP cloud load balancer for you, and start routing traffic to the selected service inside your cluster. In other words, you get ingress to an otherwise internal service.

The inlets-operator brings that same experience to your local Kubernetes cluster by provisioning a VM on the public cloud and running an [inlets server](https://inlets.dev/) process there.

Within the cluster, it runs the [inlets client](https://inlets.dev/) as a Deployment, and once the two are connected, it updates the original service with the IP, just like a managed Kubernetes engine.

Deleting the service or annotating it will cause the cloud VM to be deleted.

See also:

* [Installation for different cloud providers](https://docs.inlets.dev/reference/inlets-operator/)
* [Expose an Ingress Controller or Istio](#expose-an-ingress-controller-or-istio-ingress-gateway)
* [Helm chart](/chart/inlets-operator/)
* [Conceptual overview](#conceptual-overview)

## Change any LoadBalancer from `<pending>` to a real IP

Once the inlets-operator is installed, any Service of type LoadBalancer will get an IP address, unless you exclude it with an annotation.

```bash
kubectl run nginx-1 --image=nginx --port=80 --restart=Always
kubectl expose pod/nginx-1 --port=80 --type=LoadBalancer

$ kubectl get services -w
NAME               TYPE        CLUSTER-IP        EXTERNAL-IP       PORT(S)   AGE
service/nginx-1    ClusterIP   192.168.226.216   <pending>         80/TCP    78s
service/nginx-1    ClusterIP   192.168.226.216   104.248.163.242   80/TCP    78s
```

You'll also find a Tunnel Custom Resource created for you:

```bash
$ kubectl get tunnels

NAMESPACE   NAME             SERVICE   HOSTSTATUS     HOSTIP         HOSTID
default     nginx-1-tunnel   nginx-1   provisioning                  342453649
default     nginx-1-tunnel   nginx-1   active         178.62.64.13   342453649
```

We recommend exposing an Ingress Controller or Istio Ingress Gateway, see also: [Expose an Ingress Controller](#expose-an-ingress-controller-or-istio-ingress-gateway)

## Plays well with other LoadBalancers

Want to create tunnels for all LoadBalancer services, but ignore one or two?

Want to disable the inlets-operator for a particular Service? Add the annotation `operator.inlets.dev/manage` with a value of `0`.

```bash
kubectl annotate service nginx-1 operator.inlets.dev/manage=0
```

Want to ignore all services, then only create Tunnels for annotated ones?

Install the chart with `annotatedOnly: true`, then run:

```bash
kubectl annotate service nginx-1 operator.inlets.dev/manage=1
```

## Using IPVS for your Kubernetes networking?

For IPVS, you need to declare a Tunnel Custom Resource instead of using the LoadBalancer field.

```yaml
apiVersion: operator.inlets.dev/v1alpha1
kind: Tunnel
metadata:
  name: nginx-1-tunnel
  namespace: default
spec:
  serviceRef:
    name: nginx-1
    namespace: default
status: {}
```

You can pre-define the auth token for the tunnel if you need to:

```yaml
spec:
  authTokenRef:
    name: nginx-1-tunnel-token
    namespace: default
```

## Who is this for?

Your cluster could be running anywhere: on your laptop, in an on-premises datacenter, within a VM, or on your Raspberry Pi. Ingress and LoadBalancers are a core-building block of Kubernetes clusters, so Ingress is especially important if you:

* run a private-cloud or a homelab
* self-host applications and APIs
* test and share work with colleagues or clients
* want to build a realistic environment
* integrate with webhooks and third-party APIs

There is no need to open a firewall port, set-up port-forwarding rules, configure dynamic DNS or any of the usual hacks. You will get a public IP and it will "just work" for any TCP traffic you may have.

## How does it compare to other solutions?

* There are no rate limits on connections or bandwidth limits
* You can use your own DNS
* You can use any IngressController or an Istio Ingress Gateway
* You can take your IP address with you - wherever you go

Any Service of type `LoadBalancer` can be exposed within a few seconds.

Since exit-servers are created in your preferred cloud (around a dozen are supported already), you'll only have to pay for the cost of the VM, and where possible, the cheapest plan has already been selected for you. For example with Hetzner ([coming soon](https://github.com/inlets/inlets-operator/issues/115)) that's about 3 EUR / mo, and with DigitalOcean it comes in at around 5 USD - both of these VPSes come with generous bandwidth allowances, global regions and fast network access.

## Conceptual overview

In this animation by [Ivan Velichko](https://iximiuz.com/en/posts/kubernetes-operator-pattern), you see the operator in action.

It detects a new Service of type LoadBalancer, provisions a VM in the cloud, and then updates the Service with the IP address of the VM.

[![Demo GIF](https://iximiuz.com/kubernetes-operator-pattern/kube-operator-example-opt.gif)](https://iximiuz.com/en/posts/kubernetes-operator-pattern)

There's also a [video walk-through of exposing an Ingress Controller](https://www.youtube.com/watch?v=4wFSdNW-p4Q)

## Installation

Read the [installation instructions for different cloud providers](https://docs.inlets.dev/reference/inlets-operator/)

The image for this operator is multi-arch and supports both `x86_64` and `arm64`.

See also: [Helm chart](/chart/inlets-operator/)

### Expose an Ingress Controller or Istio Ingress Gateway

Unlike other solutions, this:

* Integrates directly into Kubernetes
* Gives you a TCP LoadBalancer, and updates its IP in `kubectl get svc`
* Allows you to use any custom DNS you want
* Works with LetsEncrypt

Configuring ingress:

* [Expose Ingress Nginx or another IngressController](https://docs.inlets.dev/tutorial/kubernetes-ingress/)
* [Expose an Istio Ingress Gateway](https://docs.inlets.dev/tutorial/istio-gateway/)
* [Expose Traefik with K3s to the Internet](https://inlets.dev/blog/2021/12/06/expose-traefik.html)

### Other use-cases

* [Node.js microservice with Let's Encrypt](https://docs.inlets.dev/#/get-started/quickstart-ingresscontroller-cert-manager?id=expose-your-ingresscontroller-and-get-tls-from-letsencrypt)
* [OpenFaaS with Let's Encrypt](https://inlets.dev/blog/2020/10/15/openfaas-public-endpoints.html)
* [Docker Registry with Let's Encrypt](https://blog.alexellis.io/get-a-tls-enabled-docker-registry-in-5-minutes/)

# Provider Pricing

The host [provisioning code](https://github.com/inlets/cloud-provision) used by the inlets-operator is shared with [inletsctl](https://github.com/inlets/inletsctl), both tools use the configuration in the grid below.

These costs need to be treated as an estimate and will depend on your bandwidth usage and how many hosts you decide to create. You can at all times check your cloud provider's dashboard, API, or CLI to view your exit-nodes. The hosts provided have been chosen because they are the absolute lowest-cost option that the maintainers could find.

| Provider                                                           | Price per month | Price per hour |     OS image | CPU | Memory | Boot time |
| ------------------------------------------------------------------ | --------------: | -------------: | -----------: | --: | -----: | --------: |
| [Google Compute Engine](https://cloud.google.com/compute)          |      *  ~\$4.28 |       ~\$0.006 | Ubuntu 22.04 | 1 | 614MB | ~3-15s |
| [Digital Ocean](https://www.digitalocean.com/pricing/#Compute)     |             \$5 |      ~\$0.0068 | Ubuntu 22.04 |   1 |  1GB   | ~20-30s  |
| [Scaleway](https://www.scaleway.com/en/pricing/#virtual-instances) |           5.84€ |         0.01€ | Ubuntu 22.04 |   2 |    2GB | 3-5m     |
| [Amazon Elastic Computing 2](https://calculator.aws/#/createCalculator/EC2s) |           $3.796 |         $0.0052 | Ubuntu 20.04 |   1 |    1GB | 3-5m     |
| [Linode](https://www.linode.com/pricing/) |           $5 |         $0.0075 | Ubuntu 22.04 |   1 |    1GB | ~10-30s    |
| [Azure](https://azureprice.net/?cores=1,1&ram=0,11400) |           $4.53	 |         $0.0062 | Ubuntu 22.04 |   1 |    0.5GB | 2-4min    |
| [Hetzner](https://www.hetzner.com/cloud) |           4.15€	 |         €0.007 | Ubuntu 22.04 |   1 |    2GB | ~5-10s    |

* The first f1-micro instance in a GCP Project (the default instance type for inlets-operator) is free for 720hrs(30 days) a month

## Video walk-through

In this video walk-through Alex will guide you through creating a Kubernetes cluster on your laptop with KinD, then he'll install ingress-nginx (an IngressController), followed by cert-manager and then after the inlets-operator creates a LoadBalancer on the cloud, you'll see a TLS certificate obtained by LetsEncrypt.

[![Video demo](https://img.youtube.com/vi/4wFSdNW-p4Q/hqdefault.jpg)](https://www.youtube.com/watch?v=4wFSdNW-p4Q)

Tutorial: [Tutorial: Expose a local IngressController with the inlets-operator](https://docs.inlets.dev/tutorial/kubernetes-ingress/)

## Contributing

Contributions are welcome, see the [CONTRIBUTING.md](CONTRIBUTING.md) guide.

## Also in this space

- [inlets](https://inlets.dev) - L7 HTTP / L4 TCP tunnel which can tunnel any TCP traffic. One of the ways to deploy it is via the inlets-operator.
- [MetalLB](https://github.com/metallb/metallb) - a LoadBalancer for private Kubernetes clusters, cannot expose services publicly
- [kube-vip](https://kube-vip.io/) - a Kubernetes LoadBalancer similar to MetalLB, cannot expose services publicly
- [Cloudflare Tunnel aka "Argo"](https://www.cloudflare.com/en-gb/products/tunnel/) - product from Cloudflare for Cloudflare customers and domains - K8s integration available through Cloudflare DNS and ingress controller. Not for use with existing Ingress Controllers, unable to provide LoadBalancer
- [ngrok](https://ngrok.com) - a SasS tunnel service tool, restarts every 7 hours, limits connections per minute, SaaS-only, no K8s integration available, TCP tunnels can only use high/unconventional ports, can't be used with Ingress Controllers
- [Wireguard](https://www.wireguard.com/) - a modern VPN for connecting whole hosts and networks. Does not expose HTTP or TCP ports publicly.
- [Tailscale](https://tailscale.com/) - a managed SaaS VPN that is built upon Wireguard.

## Author / vendor

inlets and the inlets-operator are brought to you by [OpenFaaS Ltd](https://www.openfaas.com).
