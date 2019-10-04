# inlets-operator

Get a Kubernetes LoadBalancer where you never thought it was possible.

In cloud-based Kubernetes solutions, Services can be exposed as type "LoadBalancer" and your cloud provider will provision a LoadBalancer and start routing traffic, in another word: you get ingress to your service.

inlets-operator brings that same experience to your local Kubernetes or k3s cluster (k3s/k3d/minikube/microk8s/Docker Desktop/KinD). The operator automates the creation of an [inlets](https://inlets.dev) exit-node on public cloud, and runs the client as a Pod inside your cluster. Your Kubernetes `Service` will be updated with the public IP of the exit-node and you can start receiving incoming traffic immediately.

## Who is this for?

This solution is for users who want to gain incoming network access (ingress) to their private Kubernetes clusters running on their laptops, VMs, within a Docker container, on-premises, or behind NAT. The cost of the LoadBalancer with a IaaS like DigitalOcean is around 5 USD / mo, which is 10 USD cheaper than an AWS ELB or GCP LoadBalancer.

Whilst 5 USD is cheaper than a "Cloud Load Balancer", this tool is for users who cannot get incoming ingress, not for saving money on public cloud.

## Status

This version of the inlets-operator is a early proof-of-concept, but it builds upon inlets, which is stable and widely used.

Backlog:
- [x] Provision VMs/exit-nodes on public cloud
- [x] Provision to [Packet.com](https://packet.com)
- [x] Provision to DigitalOcean
- [x] Automatically update Service type LoadBalancer with a public IP
- [x] Tunnel `http` traffic
- [x] In-cluster Role, Dockerfile and YAML files
- [ ] Automate `wss://` for control-port
- [ ] Move control-port and `/tunnel` endpoint to high port i.e. `31111`
- [ ] Garbage collect hosts when CRD is deleted
- [ ] Provision to EC2
- [ ] Provision to GCP
- [ ] Tunnel any `tcp` traffic (using `inlets-pro`)

## Video demo

[![https://img.youtube.com/vi/LeKMSG7QFSk/0.jpg](https://img.youtube.com/vi/LeKMSG7QFSk/0.jpg)](https://www.youtube.com/watch?v=LeKMSG7QFSk&amp=&feature=youtu.be)

Watch me get a LoadBalancer with a public IP for my KinD cluster and Nginx which is running there.

## Try with Packet.com

Assuming you're running a local cluster with [KinD](https://github.com/kubernetes-sigs/kind):

Sign up to [Packet.com](https://packet.com) and get an access key, save it in `~/packet-token`

```sh
kubectl apply ./aritifacts/crd.yaml

export PACKET_PROJECT_ID=""	# Populate from dashboard

export GOPATH=$HOME/go/
go get -u github.com/alexellis/inlets-operator
cd $GOPATH/github.com/alexellis/inlets-operator

go get

go build && ./inlets-operator  --kubeconfig "$(kind get kubeconfig-path --name="kind")" --access-key=$(cat ~/packet-token) --project-id="${PACKET_PROJECT_ID}"
```

## Try with DigitalOcean

Assuming you're running a local cluster with [KinD](https://github.com/kubernetes-sigs/kind):

Sign up to [DigitalOcean.com](https://DigitalOcean.com) and get an access key, save it in `~/do-access-token`.

```sh
kubectl apply ./aritifacts/crd.yaml

export GOPATH=$HOME/go/
go get -u github.com/alexellis/inlets-operator
cd $GOPATH/github.com/alexellis/inlets-operator

go get

go build && ./inlets-operator  --kubeconfig "$(kind get kubeconfig-path --name="kind")" --access-key=$(cat ~/do-access-token) --provider digitalocean
```

See a video demo of [DigitalOcean](https://youtu.be/c6DTrNk9zRk).

## Running in-cluster

You can also run the operator in-cluster, a ClusterRole is used since Services can be created in any namespace, and may need a tunnel.

```sh
# Create a secret to store the access token

kubectl create secret generic inlets-access-key \
  --from-literal inlets-access-key="$(cat ~/Downloads/do-access-token)"

# Apply the operator deployment and RBAC role
kubectl apply -f ./artifacts/operator-rbac.yaml
kubectl apply -f ./artifacts/operator-amd64.yaml
```

# Monitor/view logs
kubectl logs deploy/inlets-operator -f
```

## Get a LoadBalancer provided by inlets

```sh
kubectl run nginx-1 --image=nginx --port=80 --restart=Always
kubectl run nginx-2 --image=nginx --port=80 --restart=Always

kubectl expose deployment nginx-1 --port=80 --type=LoadBalancer
kubectl expose deployment nginx-2 --port=80 --type=LoadBalancer

kubectl get svc

kubectl get tunnel nginx-tunnel-1 -o yaml

kubectl get svc

kubectl logs deploy/nginx-1-tunnel-client
```

Check the IP of the LoadBalancer and then access it via the Internet.

Example with OpenFaaS, make sure you give the port a name of `http`:

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
