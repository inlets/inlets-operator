/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/alexellis/inlets-operator/pkg/generated/clientset/versioned"
	informers "github.com/alexellis/inlets-operator/pkg/generated/informers/externalversions"
	"github.com/alexellis/inlets-operator/pkg/signals"
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
	AccessKeyFile     string
	ProjectID         string
	InletsClientImage string
}

// GetInletsClientImage returns the image for the client-side tunnel
func (i *InfraConfig) GetInletsClientImage() string {
	if i.InletsClientImage == "" {
		return "alexellis2/inlets:2.4.1"
	}
	return i.InletsClientImage
}

// GetAccessKey from parameter or file
func (i *InfraConfig) GetAccessKey() string {
	if len(i.AccessKeyFile) > 0 {
		data, err := ioutil.ReadFile(i.AccessKeyFile)

		if err != nil {
			log.Fatalln(err)
		}
		return string(data)
	}

	return i.AccessKey
}

func main() {
	infra := &InfraConfig{}
	flag.StringVar(&infra.Provider, "provider", "packet", "Your infrastructure provider - 'packet' or 'digitalocean'")
	flag.StringVar(&infra.Region, "region", "ams1", "The region to provision hosts into")
	flag.StringVar(&infra.AccessKey, "access-key", "", "The access key for your infrastructure provider")
	flag.StringVar(&infra.AccessKeyFile, "access-key-file", "", "Read the access key for your infrastructure provider from a file (recommended)")

	flag.StringVar(&infra.ProjectID, "project-id", "", "The project ID if using Packet.com as the provider")

	flag.Parse()

	infra.InletsClientImage = os.Getenv("client_image")

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

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
