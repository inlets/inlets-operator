module github.com/inlets/inlets-operator

go 1.15

// replace github.com/inlets/inletsctl => /home/alex/inletsctl

require (
	github.com/aws/aws-sdk-go v1.27.3 // indirect
	github.com/inlets/cloud-provision/provision v0.0.0-20210707085044-93cc13af558a
	github.com/sethvargo/go-password v0.2.0
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v0.18.3
	k8s.io/code-generator v0.18.5
	k8s.io/gengo v0.0.0-20200127102705-1e9b17e831be // indirect
	k8s.io/klog v1.0.0
)
