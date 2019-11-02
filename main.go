package main

import (
	"flag"
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
)

var (
	masterURL  string
	kubeconfig string
)

// InfraConfig is the configuration for
// creating Infrastructure Resources
type InfraConfig struct {
	Provider          string
	Region            string
	AccessKey         string
	SecretKey         string
	OrganizationID    string
	AccessKeyFile     string
	SecretKeyFile     string
	ProjectID         string
	InletsClientImage string
	AnnotatedOnly     bool
	ProConfig         InletsProConfig
}

func (i InfraConfig) UsePro() bool {
	return len(i.ProConfig.License) > 0
}

type InletsProConfig struct {
	License string
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}

func main() {
	infra := &InfraConfig{
		ProConfig: InletsProConfig{},
	}

	flag.StringVar(&infra.Provider, "provider", "packet", "Your infrastructure provider - 'packet' or 'digitalocean'")
	flag.StringVar(&infra.Region, "region", "", "The region to provision hosts into")
	flag.StringVar(&infra.AccessKey, "access-key", "", "The access key for your infrastructure provider")
	flag.StringVar(&infra.AccessKeyFile, "access-key-file", "", "Read the access key for your infrastructure provider from a file (recommended)")
	flag.StringVar(&infra.SecretKey, "secret-key", "", "The secret key if using Scaleway as the provider")
	flag.StringVar(&infra.SecretKeyFile, "secret-key-file", "", "Read the access key for your infrastructure provider from a file (recommended)")
	flag.StringVar(&infra.OrganizationID, "organization-id", "", "The organization id if using Scaleway as the provider")
	flag.StringVar(&infra.ProjectID, "project-id", "", "The project ID if using Packet.com as the provider")
	flag.StringVar(&infra.ProConfig.License, "license", "", "Supply a license for use with inlets-pro")
	flag.BoolVar(&infra.AnnotatedOnly, "annotated-only", false, "Only create a tunnel for annotated services. Annotate with dev.inlets.manage=true.")

	flag.Parse()

	infra.InletsClientImage = os.Getenv("client_image")

	log.Printf("Inlets client: %s\n", infra.GetInletsClientImage())
	log.Printf("Inlets pro: %v\n", infra.UsePro())

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
	exampleInformerFactory := informers.NewSharedInformerFactory(operatorClient, time.Second*30)

	controller := NewController(kubeClient, operatorClient,
		kubeInformerFactory.Apps().V1().Deployments(),
		exampleInformerFactory.Inletsoperator().V1alpha1().Tunnels(),
		kubeInformerFactory.Core().V1().Services(),
		infra)

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(stopCh)
	exampleInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

// GetInletsClientImage returns the image for the client-side tunnel
func (i *InfraConfig) GetInletsClientImage() string {
	if i.UsePro() {
		return "alexellis2/inlets-pro:0.4"
	}
	if i.InletsClientImage == "" {
		return "inlets/inlets:2.6.1"
	}

	return i.InletsClientImage
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
