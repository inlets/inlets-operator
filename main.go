package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/inlets/inlets-operator/pkg/generated/clientset/versioned"
	informers "github.com/inlets/inlets-operator/pkg/generated/informers/externalversions"
	"github.com/inlets/inlets-operator/pkg/signals"
	"github.com/inlets/inlets-operator/pkg/version"

	// required for generating code from CRD
	_ "k8s.io/code-generator/cmd/client-gen/generators"
)

var (
	masterURL  string
	kubeconfig string
)

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

func main() {
	infra := &InfraConfig{
		ProConfig: InletsProConfig{},
	}

	flag.StringVar(&infra.Provider, "provider", "equinix-metal", "Your infrastructure provider - 'equinix-metal', 'digitalocean', 'scaleway', 'gce', 'linode', 'azure', 'ec2' or 'hetzner'")
	flag.StringVar(&infra.Region, "region", "", "The region to provision hosts into")
	flag.StringVar(&infra.Zone, "zone", "us-central1-a", "The zone where the exit node is to be provisioned")
	flag.StringVar(&infra.AccessKey, "access-key", "", "The access key for your infrastructure provider")
	flag.StringVar(&infra.AccessKeyFile, "access-key-file", "", "Read the access key for your infrastructure provider from a file (recommended)")
	flag.StringVar(&infra.SecretKey, "secret-key", "", "The secret key if using scaleway or ec2 as the provider")
	flag.StringVar(&infra.SecretKeyFile, "secret-key-file", "", "Read the access key for your infrastructure provider from a file (recommended)")
	flag.StringVar(&infra.SubscriptionID, "subscription-id", "", "The azure Subscription ID")
	flag.StringVar(&infra.OrganizationID, "organization-id", "", "The organization id if using scaleway as the provider")
	flag.StringVar(&infra.VpcID, "vpc-id", "", "The VPC ID to create the exit-server in (ec2)")
	flag.StringVar(&infra.SubnetID, "subnet-id", "", "The Subnet ID where the exit-server should be placed (ec2)")
	flag.StringVar(&infra.ProjectID, "project-id", "", "The project ID if using equinix-metal, or gce as the provider")
	flag.StringVar(&infra.ProConfig.License, "license", "", "Supply a license for use with inlets-pro")
	flag.StringVar(&infra.ProConfig.LicenseFile, "license-file", "", "Supply a file to read for the inlets-pro license")
	flag.StringVar(&infra.ProConfig.ClientImage, "pro-client-image", "", "Supply a Docker image for the inlets-pro client")
	flag.StringVar(&infra.MaxClientMemory, "max-client-memory", "128Mi", "Maximum memory limit for the tunnel clients")

	flag.BoolVar(&infra.AnnotatedOnly, "annotated-only", false, "Only create a tunnel for annotated services. Annotate with dev.inlets.manage=true.")

	flag.Parse()
	log.Printf("Operator version: %s SHA: %s\n", version.Release, version.SHA)

	err := validateFlags(*infra)

	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	infra.InletsClientImage = os.Getenv("client_image")

	log.Printf("Client image: %s\n", infra.GetInletsClientImage())

	if _, err := infra.ProConfig.GetLicenseKey(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	operatorClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	tunnelsInformerFactory := informers.NewSharedInformerFactory(operatorClient, time.Second*30)

	controller := NewController(kubeClient, operatorClient,
		kubeInformerFactory.Apps().V1().Deployments(),
		tunnelsInformerFactory.Inlets().V1alpha1().Tunnels(),
		kubeInformerFactory.Core().V1().Services(),
		infra)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	tunnelsInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

// GetInletsClientImage returns the image for the client-side tunnel
func (i *InfraConfig) GetInletsClientImage() string {
	if i.ProConfig.ClientImage == "" {
		return "ghcr.io/inlets/inlets-pro:0.8.5"
	}
	return i.ProConfig.ClientImage
}

// GetAccessKey from parameter or file trimming
// any whitespace found.
func (i *InfraConfig) GetAccessKey() string {
	if len(i.AccessKeyFile) > 0 {
		data, err := ioutil.ReadFile(i.AccessKeyFile)

		if err != nil {
			log.Fatalln(err)
		}

		return strings.TrimSpace(string(data))
	}

	return strings.TrimSpace(i.AccessKey)
}

// GetSecretKey from parameter or file trimming
// any whitespace found.
func (i *InfraConfig) GetSecretKey() string {
	if len(i.SecretKeyFile) > 0 {
		data, err := ioutil.ReadFile(i.SecretKeyFile)

		if err != nil {
			log.Fatalln(err)
		}

		return strings.TrimSpace(string(data))
	}

	return strings.TrimSpace(i.SecretKey)
}
