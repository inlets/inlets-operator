module github.com/inlets/inlets-operator

go 1.13

// replace github.com/inlets/inletsctl => /home/alex/inletsctl

require (
	github.com/aws/aws-sdk-go v1.27.3 // indirect
	github.com/inlets/inletsctl v0.0.0-20201201134944-21dadc6da37e

	github.com/sethvargo/go-password v0.2.0

	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v0.18.3
	k8s.io/code-generator v0.18.5
	k8s.io/gengo v0.0.0-20200127102705-1e9b17e831be // indirect
	k8s.io/klog v1.0.0
)
