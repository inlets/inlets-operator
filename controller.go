// Copyright (c) inlets Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	password "github.com/sethvargo/go-password/password"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appsinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	provision "github.com/inlets/cloud-provision/provision"

	inletsv1alpha1 "github.com/inlets/inlets-operator/pkg/apis/inletsoperator/v1alpha1"
	clientset "github.com/inlets/inlets-operator/pkg/generated/clientset/versioned"
	inletsscheme "github.com/inlets/inlets-operator/pkg/generated/clientset/versioned/scheme"
	informers "github.com/inlets/inlets-operator/pkg/generated/informers/externalversions/inletsoperator/v1alpha1"
	listers "github.com/inlets/inlets-operator/pkg/generated/listers/inletsoperator/v1alpha1"
)

const controllerAgentName = "inlets-operator"
const inletsPROControlPort = 8123
const inletsPortsAnnotation = "inlets.dev/ports"
const licenseSecretName = "inlets-license"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Tunnel is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Tunnel fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Tunnel"
	// MessageResourceSynced is the message used for an Event fired when a Tunnel
	// is synced successfully
	MessageResourceSynced = "Tunnel synced successfully"
)

// Controller is the controller implementation for Tunnel resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	operatorclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	tunnelsLister     listers.TunnelLister
	tunnelsSynced     cache.InformerSynced
	serviceLister     corelisters.ServiceLister
	infraConfig       *InfraConfig

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new controller
func NewController(
	kubeclientset kubernetes.Interface,
	operatorClient clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	tunnelInformer informers.TunnelInformer,
	serviceInformer coreinformers.ServiceInformer,
	infra *InfraConfig,
) *Controller {

	utilruntime.Must(inletsscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.V(4).Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:     kubeclientset,
		operatorclientset: operatorClient,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		tunnelsLister:     tunnelInformer.Lister(),
		tunnelsSynced:     tunnelInformer.Informer().HasSynced,
		serviceLister:     serviceInformer.Lister(),
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Tunnels"),
		recorder:          recorder,
		infraConfig:       infra,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Tunnel resources change
	tunnelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new interface{}) {
			controller.enqueueTunnel(new)
		},
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueTunnel(new)
		},
		DeleteFunc: func(old interface{}) {
			r, ok := checkCustomResourceType(old)

			klog.Infof("Deleting Tunnel: %s.%s, references: %v", r.Name, r.Namespace, r.OwnerReferences)

			if !ok {
				log.Println("Failed to retrieve resource status")
				return
			}
			if len(r.Status.HostID) == 0 {
				log.Println("Status.HostID is empty")
				return
			}
			provisioner, err := getProvisioner(controller)
			if err != nil {
				klog.Infof("Error creating provisioner: %s", err.Error())
				return
			}

			if provisioner != nil {
				klog.Infof("Deleting tunnel server for: %s.%s, HostID: %s, IP: %s",
					r.Name, r.Namespace, r.Status.HostID, r.Status.HostIP)

				if err := provisioner.Delete(provision.HostDeleteRequest{
					ID:     r.Status.HostID,
					IP:     r.Status.HostIP,
					Region: infra.Region,
					Zone:   infra.Zone,
				}); err != nil {
					klog.Infof("Error deleting tunnel server %s", err)
					return
				}

				// This will fail if the service was deleted first
				controller.updateService(&r, "")
			}
		},
	})

	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Tunnel resource will enqueue that Tunnel resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(new interface{}) {
			if ok := checkServiceType(new); !ok {
				return
			}

			controller.enqueueService(new)
		},
		UpdateFunc: func(old, new interface{}) {
			if ok := checkServiceType(new); !ok {
				return
			}

			newSvc := new.(*corev1.Service)
			oldSvc := old.(*corev1.Service)
			if !apiequality.Semantic.DeepEqual(oldSvc.Spec, newSvc.Spec) || cmp.Diff(oldSvc.Annotations, newSvc.Annotations) != "" {
				controller.enqueueService(new)
			}
		},
	})

	return controller
}

func checkServiceType(obj interface{}) bool {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		return false
	}

	return svc.Spec.Type == corev1.ServiceTypeLoadBalancer
}

func checkCustomResourceType(obj interface{}) (inletsv1alpha1.Tunnel, bool) {
	var roll *inletsv1alpha1.Tunnel
	var ok bool
	if roll, ok = obj.(*inletsv1alpha1.Tunnel); !ok {
		return inletsv1alpha1.Tunnel{}, false
	}
	return *roll, true
}

// enqueueInletsLoadBalancer takes a InletsLoadBalancer resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than InletsLoadBalancer.
func (c *Controller) enqueueService(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Tunnel controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.tunnelsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	// Launch two workers to process Tunnel resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Tunnel resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Tunnel resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	service, err := c.serviceLister.Services(namespace).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error listing services: %s", err)
	}

	// A service is causing this event, make sure a Tunnel CR exists
	if service != nil {

		if err := createTunnelResource(service, c); err != nil {
			return fmt.Errorf("error creating tunnel: %s", err)
		}

		return nil
	}

	// A tunnel CR is causing this event
	tunnel, err := c.tunnelsLister.Tunnels(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

		return err
	}

	// The tunnel CR is invalid without a service reference
	if tunnel.Spec.ServiceRef == nil {
		return fmt.Errorf("tunnel %s.%s has no service reference", tunnel.Name, tunnel.Namespace)
	}

	switch tunnel.Status.HostStatus {
	case "":

		// No pre-created secret ref, and no generated secret name either
		// so create one.
		if getSecretName(tunnel) == "" {
			_, err = createTunnelAuthTokenSecret(tunnel, c)
			if err != nil {
				klog.Infof("Error creating tunnel auth token: %s", err)
				return fmt.Errorf("error creating tunnel auth token: %s", err)
			}

			klog.Infof("Created tunnel auth token for %s.%s", tunnel.Name, tunnel.Namespace)
			// The status will be updated by the call to create an auth secret, therefore
			// return, and let that event handle the provisioning
			return nil
		}

		provisioner, err := getProvisioner(c)
		if err != nil {
			return err
		}

		if tunnel.Spec.ServiceRef == nil {
			return fmt.Errorf("tunnel %s.%s has no service reference", tunnel.Name, tunnel.Namespace)
		}

		if service == nil {
			svc, err := c.serviceLister.Services(tunnel.Namespace).Get(tunnel.Spec.ServiceRef.Name)
			if err != nil {
				return fmt.Errorf("error getting service: %s", err)
			}
			service = svc
		}

		start := time.Now()
		hostConfig, err := getHostConfig(c,
			tunnel,
			service,
			c.infraConfig.Plan,
			c.infraConfig.GetInletsRelease())
		if err != nil {
			return fmt.Errorf("error building host config: %s", err)
		}

		res, err := provisioner.Provision(hostConfig)
		if err != nil {
			return err
		}

		klog.Infof("Provisioning for %s.%s took: %fs\n", tunnel.Name, tunnel.Namespace, time.Since(start).Seconds())

		copy := tunnel.DeepCopy()

		// Update Status
		if _, err := c.updateTunnelProvisioningStatus(copy, "provisioning", res.ID, ""); err != nil {
			return fmt.Errorf("tunnel %s.%s (%s) update error: %s", tunnel.Name, tunnel.Namespace, "provisioning", err)
		}

	case "provisioning":

		// If an error occurs during Update, we'll requeue the item so we can
		// attempt processing again later. THis could have been caused by a
		// temporary network failure, or any other transient reason.
		if err := syncProvisioningHostStatus(tunnel, c); err != nil {
			return err
		}

	case provision.ActiveStatus:

		operatorNs := readNamespace()
		if err := syncTunnelLicense(c, operatorNs, namespace); err != nil {
			return fmt.Errorf("error creating tunnel license in %s: %s", namespace, err)
		}

		err := createClientDeployment(tunnel, c)
		if err != nil {
			return fmt.Errorf("error creating client deployment: %s", err)
		}
	}

	c.recorder.Event(tunnel, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func syncTunnelLicense(c *Controller, operatorNs string, namespace string) error {
	licenseToken, err := c.kubeclientset.CoreV1().
		Secrets(operatorNs).
		Get(context.Background(), "inlets-license", metav1.GetOptions{})
	if err != nil || licenseToken == nil {
		return fmt.Errorf("unable to read inlets license: %s", err.Error())
	}

	existing, err := c.kubeclientset.CoreV1().
		Secrets(namespace).
		Get(context.Background(), "inlets-license", metav1.GetOptions{})
	if err == nil {
		if cmp.Equal(existing.Data["license"], licenseToken.Data["license"]) {
			return nil
		}

		copyExisting := existing.DeepCopy()
		copyExisting.Data = licenseToken.Data

		_, err = c.kubeclientset.CoreV1().Secrets(namespace).
			Update(context.Background(), copyExisting, metav1.UpdateOptions{})
		return err
	}

	licenseCopy := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "inlets-license",
			Namespace: namespace,
		},
		Data: licenseToken.Data,
	}

	if _, err = c.kubeclientset.CoreV1().Secrets(namespace).
		Create(context.Background(), &licenseCopy, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("unable to create inlets license in %s, error: %s", namespace, err.Error())
	}

	return nil
}

func readNamespace() string {
	namespace := "default"

	if ns, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		namespace = string(ns)
	} else if v := os.Getenv("NAMESPACE"); v != "" {
		namespace = v
	}

	return namespace
}

func createTunnelAuthTokenSecret(tunnel *inletsv1alpha1.Tunnel, c *Controller) (*inletsv1alpha1.Tunnel, error) {
	name := tunnel.Name
	namespace := tunnel.Namespace

	if _, err := c.kubeclientset.CoreV1().
		Secrets(tunnel.Namespace).
		Get(context.Background(), tunnel.Name, metav1.GetOptions{}); err != nil && errors.IsNotFound(err) {

		pwdRes, err := password.Generate(64, 10, 0, false, true)
		if err != nil {
			return tunnel, fmt.Errorf("unable to generate password for server: %s", err.Error())
		}

		// create secret in cluster

		authSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(tunnel, schema.GroupVersionKind{
						Group:   "operator.inlets.dev",
						Version: "v1alpha1",
						Kind:    "Tunnel",
					}),
				},
			},
			Data: map[string][]byte{
				"token": []byte(pwdRes),
			},
		}

		_, err = c.kubeclientset.CoreV1().
			Secrets(namespace).
			Create(context.Background(), authSecret, metav1.CreateOptions{})

		if err != nil && !errors.IsAlreadyExists(err) {
			klog.Infof("Error creating secret: %s", err.Error())
			return tunnel, fmt.Errorf("unable to create secret: %s", err.Error())
		}

	}

	tunnelCopy := tunnel.DeepCopy()
	tunnelCopy.Status.AuthTokenRef = &inletsv1alpha1.ResourceRef{
		Name:      name,
		Namespace: namespace,
	}

	var err error
	tunnel, err = c.operatorclientset.OperatorV1alpha1().
		Tunnels(namespace).
		UpdateStatus(context.Background(), tunnelCopy, metav1.UpdateOptions{})
	if err != nil {
		return tunnel, fmt.Errorf("unable to update tunnel status with auth token: %s", err.Error())
	}

	return tunnel, nil
}

func createTunnelResource(service *corev1.Service, c *Controller) error {
	name := service.Name + "-tunnel"
	namespace := service.Namespace

	tunnels := c.operatorclientset.OperatorV1alpha1().
		Tunnels(service.ObjectMeta.Namespace)

	ops := metav1.GetOptions{}

	found, err := tunnels.Get(context.Background(), name, ops)

	// Create Tunnel CR
	if errors.IsNotFound(err) {

		if !manageService(*c, *service) {
			return nil
		}

		klog.Infof("Creating Tunnel: %s.%s\n", name, namespace)

		tunnel := &inletsv1alpha1.Tunnel{
			Spec: inletsv1alpha1.TunnelSpec{
				ServiceRef: &inletsv1alpha1.ResourceRef{
					Name:      service.Name,
					Namespace: service.Namespace,
				},
				UpdateServiceIP: true,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: service.ObjectMeta.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(service, schema.GroupVersionKind{
						Group:   "",
						Version: "v1",
						Kind:    "Service",
					}),
				},
			},
		}

		ops := metav1.CreateOptions{}
		created, err := tunnels.Create(context.Background(), tunnel, ops)
		if err != nil {
			klog.Infof("Error creating Tunnel: %s.%s", err.Error(), tunnel.Namespace)
		}

		copy := created.DeepCopy()
		copy.Status.Generated = true
		if _, err := tunnels.UpdateStatus(context.Background(), copy, metav1.UpdateOptions{}); err != nil {
			klog.Infof("Error updating Tunnel status: %s.%s", err.Error(), copy.Namespace)
		}

		return nil
	} else {
		// klog.Infof("Event for existing Tunnel: %s.%s\n", found.Name, found.Namespace)
		// c.enqueueTunnel(found)

		if !manageService(*c, *service) {
			klog.Infof("Removing Tunnel: %s.%s, no longer managed by controller", found.Name, found.Namespace)

			if err := tunnels.Delete(context.Background(), found.Name, metav1.DeleteOptions{}); err != nil {
				klog.Infof("Error deleting Tunnel: %s", err.Error())
			}
		}
	}

	return nil
}

func createClientDeployment(tunnel *inletsv1alpha1.Tunnel, c *Controller) error {
	if tunnel.Status.ClientDeploymentRef != nil && tunnel.Status.ClientDeploymentRef.Name != "" {
		return updateClientDeploymentRef(tunnel, c)
	}

	get := metav1.GetOptions{}
	service, err := c.kubeclientset.CoreV1().
		Services(tunnel.Namespace).
		Get(context.Background(), tunnel.Spec.ServiceRef.Name, get)
	if err != nil {
		return err
	}

	licenseKey, _ := c.infraConfig.ProConfig.GetLicenseKey()

	ports := getPortsString(service)

	client := makeClientDeployment(tunnel,
		c.infraConfig.GetInletsClientImage(),
		ports,
		licenseKey,
		c.infraConfig.MaxClientMemory)

	deployment, err := c.kubeclientset.AppsV1().
		Deployments(tunnel.Namespace).
		Create(context.Background(), client, metav1.CreateOptions{})

	if err != nil {
		klog.Infof("Failed creating deployment: %s.%s, error: %s", tunnel.Name, tunnel.Namespace, err)
	}

	tunnel.Status.ClientDeploymentRef = &inletsv1alpha1.ResourceRef{
		Name:      deployment.ObjectMeta.Name,
		Namespace: deployment.ObjectMeta.Namespace,
	}

	copy := tunnel.DeepCopy()

	if _, err := c.operatorclientset.OperatorV1alpha1().
		Tunnels(tunnel.Namespace).
		UpdateStatus(context.Background(), copy, metav1.UpdateOptions{}); err != nil {
		klog.Infof("Failed updating tunnel: %s.%s, error: %s",
			tunnel.Name, tunnel.Namespace, err)

		return fmt.Errorf("tunnel update error %s", err)
	}

	klog.Infof("Created tunnel client deployment: %s.%s", deployment.ObjectMeta.Name, deployment.ObjectMeta.Namespace)

	return nil

}

func updateClientDeploymentRef(tunnel *inletsv1alpha1.Tunnel, c *Controller) error {
	get := metav1.GetOptions{}

	name := tunnel.Status.ClientDeploymentRef.Name
	namespace := tunnel.Status.ClientDeploymentRef.Namespace

	deployment, err := c.kubeclientset.AppsV1().
		Deployments(namespace).
		Get(context.Background(), name, get)
	if err != nil {
		return err
	}

	service, err := c.kubeclientset.CoreV1().
		Services(tunnel.Namespace).
		Get(context.Background(), tunnel.Spec.ServiceRef.Name, get)
	if err != nil {
		return err
	}

	if deployment.ObjectMeta.Annotations != nil &&
		deployment.ObjectMeta.Annotations[inletsPortsAnnotation] != getPortsString(service) {

		licenseKey, _ := c.infraConfig.ProConfig.GetLicenseKey()

		ports := getPortsString(service)
		clientDeployment := makeClientDeployment(tunnel,
			c.infraConfig.GetInletsClientImage(),
			ports,
			licenseKey,
			c.infraConfig.MaxClientMemory)

		if _, err = c.kubeclientset.AppsV1().
			Deployments(tunnel.Namespace).
			Update(context.Background(), clientDeployment, metav1.UpdateOptions{}); err != nil {
			klog.Infof("Failed to update deployment %s.%s, error: %s",
				tunnel.Name, tunnel.Namespace, err)
		}
	}

	return nil
}

func getHostConfig(c *Controller, tunnel *inletsv1alpha1.Tunnel, service *corev1.Service, planOverride, inletsVersion string) (provision.BasicHost, error) {

	tokenValue, err := getSecretValue(c, tunnel)
	if err != nil {
		klog.Infof("Error getting secret value: %s", err.Error())
		return provision.BasicHost{}, err
	}

	userData := provision.MakeExitServerUserdata(tokenValue, inletsVersion)

	var host provision.BasicHost

	inletsPort := inletsPROControlPort

	switch c.infraConfig.Provider {
	case "equinix-metal":
		host = provision.BasicHost{
			Name:     tunnel.Name,
			OS:       "ubuntu_20_04",
			Plan:     "c3.small.x86",
			Region:   c.infraConfig.Region,
			UserData: userData,
			Additional: map[string]string{
				"project_id": c.infraConfig.ProjectID,
			},
		}
	case "digitalocean":
		host = provision.BasicHost{
			Name:       tunnel.Name,
			OS:         "ubuntu-22-04-x64",
			Plan:       "s-1vcpu-1gb",
			Region:     c.infraConfig.Region,
			UserData:   userData,
			Additional: map[string]string{},
		}

	case "scaleway":
		host = provision.BasicHost{
			Name:       tunnel.Name,
			OS:         "ubuntu-focal",
			Plan:       "DEV1-S",
			Region:     c.infraConfig.Region,
			UserData:   userData,
			Additional: map[string]string{},
		}

	case "gce":
		firewallRuleName := "inlets"

		host = provision.BasicHost{
			Name:     tunnel.Name,
			OS:       "projects/ubuntu-os-cloud/global/images/ubuntu-minimal-2004-focal-v20210707",
			Plan:     "f1-micro",
			Region:   c.infraConfig.Region,
			UserData: userData,
			Additional: map[string]string{
				"projectid":     c.infraConfig.ProjectID,
				"zone":          c.infraConfig.Zone,
				"firewall-name": firewallRuleName,
				"firewall-port": strconv.Itoa(inletsPort),
			},
		}

	case "ec2":

		ports := getPortsString(service)

		var additional = map[string]string{
			"inlets-port": strconv.Itoa(inletsPort),
			"ports":       ports,
		}

		if len(c.infraConfig.VpcID) > 0 {
			additional["vpc-id"] = c.infraConfig.VpcID
		}

		if len(c.infraConfig.SubnetID) > 0 {
			additional["subnet-id"] = c.infraConfig.SubnetID
		}

		host = provision.BasicHost{
			Name:       tunnel.Name,
			OS:         "ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-20230516",
			Plan:       "t3.micro",
			UserData:   base64.StdEncoding.EncodeToString([]byte(userData)),
			Additional: additional,
		}

	case "linode":
		host = provision.BasicHost{
			Name:       tunnel.Name,
			OS:         "linode/ubuntu20.04", // https://api.linode.com/v4/images
			Plan:       "g6-nanode-1",        // https://api.linode.com/v4/linode/types
			Region:     c.infraConfig.Region,
			UserData:   userData,
			Additional: map[string]string{},
		}

	case "azure":
		// Ubuntu images can be found here https://docs.microsoft.com/en-us/azure/virtual-machines/linux/cli-ps-findimage#list-popular-images
		// An image includes more than one property, it has publisher, offer, sku and version.
		// So they have to be in "Additional" instead of just "OS".

		pro := true
		host = provision.BasicHost{
			Name:     tunnel.Name,
			OS:       "Additional.imageOffer",
			Plan:     "Standard_B1ls",
			Region:   c.infraConfig.Region,
			UserData: userData,
			Additional: map[string]string{
				"inlets-port":    strconv.Itoa(inletsPort),
				"pro":            fmt.Sprint(pro),
				"imagePublisher": "Canonical",
				"imageOffer":     "0001-com-ubuntu-server-focal",
				"imageSku":       "20_04-lts",
				"imageVersion":   "latest",
			},
		}

	case "hetzner":
		host = provision.BasicHost{
			Name:       tunnel.Name,
			OS:         "ubuntu-20.04", // https://docs.hetzner.cloud/#images-get-all-images
			Plan:       "cx11",         // https://docs.hetzner.cloud/#server-types-get-a-server-type
			Region:     c.infraConfig.Region,
			UserData:   userData,
			Additional: map[string]string{},
		}
	}

	// override default plan/size when provided
	if len(c.infraConfig.Plan) > 0 {
		host.Plan = c.infraConfig.Plan
	}
	return host, nil
}

func getSecretValue(c *Controller, tunnel *inletsv1alpha1.Tunnel) (string, error) {
	name := getSecretName(tunnel)

	secret, err := c.kubeclientset.CoreV1().
		Secrets(tunnel.Namespace).
		Get(context.Background(), name, metav1.GetOptions{})

	if err != nil {
		return "", fmt.Errorf("error getting secret %s.%s: %w", name, tunnel.Namespace, err)
	}

	token := string(secret.Data["token"])

	if len(token) == 0 {
		return "", fmt.Errorf("token value was empty")
	}

	return token, nil

}

func getProvisioner(c *Controller) (provision.Provisioner, error) {
	var err error
	var provisioner provision.Provisioner

	switch c.infraConfig.Provider {
	case "equinix-metal":
		provisioner, err = provision.NewEquinixMetalProvisioner(c.infraConfig.GetAccessKey())
	case "digitalocean":
		provisioner, err = provision.NewDigitalOceanProvisioner(c.infraConfig.GetAccessKey())
	case "scaleway":
		provisioner, err = provision.NewScalewayProvisioner(c.infraConfig.GetAccessKey(), c.infraConfig.GetSecretKey(), c.infraConfig.OrganizationID, c.infraConfig.Region)
	case "gce":
		provisioner, err = provision.NewGCEProvisioner(c.infraConfig.GetAccessKey())
	case "ec2":
		// No STS Token can be made available when running in-cluster as a service.
		emptySTSToken := ""
		provisioner, err = provision.NewEC2Provisioner(c.infraConfig.Region, c.infraConfig.GetAccessKey(), c.infraConfig.GetSecretKey(), emptySTSToken)
	case "linode":
		provisioner, err = provision.NewLinodeProvisioner(c.infraConfig.GetAccessKey())
	case "azure":
		provisioner, err = provision.NewAzureProvisioner(c.infraConfig.SubscriptionID, c.infraConfig.GetAccessKey())
	case "hetzner":
		provisioner, err = provision.NewHetznerProvisioner(c.infraConfig.GetAccessKey())
	default:
		return nil, fmt.Errorf("unsupported provider: %s", c.infraConfig.Provider)
	}
	return provisioner, err
}

func syncProvisioningHostStatus(tunnel *inletsv1alpha1.Tunnel, c *Controller) error {
	provisioner, err := getProvisioner(c)
	if err != nil {
		return err
	}

	host, err := provisioner.Status(tunnel.Status.HostID)
	if err != nil {
		return err
	}

	if host.Status != provision.ActiveStatus || host.IP == "" {
		return nil
	}

	tunnel, err = c.updateTunnelProvisioningStatus(tunnel, provision.ActiveStatus, host.ID, host.IP)
	if err != nil {
		return err
	}

	if err := c.updateService(tunnel, host.IP); err != nil {
		klog.Infof("Failed updating service %s.%s, error: %s", tunnel.Spec.ServiceRef.Name, tunnel.Namespace, err)
		return fmt.Errorf("tunnel update error %s", err)
	}

	return nil
}

func makeClientDeployment(tunnel *inletsv1alpha1.Tunnel, clientImage string, ports, license string, maxMemory string) *appsv1.Deployment {
	replicas := int32(1)
	name := tunnel.Name + "-client"

	if tunnel.Status.ClientDeploymentRef != nil && len(tunnel.Status.ClientDeploymentRef.Name) > 0 {
		name = tunnel.Status.ClientDeploymentRef.Name
	}

	container := corev1.Container{
		Name:            "inlets-client",
		Image:           clientImage,
		Command:         []string{"inlets-pro"},
		ImagePullPolicy: corev1.PullIfNotPresent,
		Args: []string{
			"tcp",
			"client",
			"--url=" + fmt.Sprintf("wss://%s:%d/connect", tunnel.Status.HostIP, inletsPROControlPort),
			"--token-file=/var/inlets/auth-token/token",
			"--upstream=" + tunnel.Spec.ServiceRef.Name,
			"--ports=" + ports,
			"--license-file=/var/inlets/license/license",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "auth-token-volume",
				MountPath: "/var/inlets/auth-token",
				ReadOnly:  true,
			},
			{
				Name:      "license-volume",
				MountPath: "/var/inlets/license",
				ReadOnly:  true,
			},
		},
	}

	container.Resources = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse(maxMemory),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("25m"),
			corev1.ResourceMemory: resource.MustParse("25Mi"),
		},
	}

	secretRef := getSecretName(tunnel)

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   tunnel.Namespace,
			Annotations: map[string]string{inletsPortsAnnotation: ports},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(tunnel, schema.GroupVersionKind{
					Group:   inletsv1alpha1.SchemeGroupVersion.Group,
					Version: inletsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Tunnel",
				}),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						container,
					},
					Volumes: []corev1.Volume{
						{
							Name: "auth-token-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: secretRef,
								},
							},
						},
						{
							Name: "license-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: licenseSecretName,
								},
							},
						},
					},
				},
			},
		},
	}

	return &deployment
}

func getSecretName(tunnel *inletsv1alpha1.Tunnel) string {
	if tunnel.Spec.AuthTokenRef != nil {
		return tunnel.Spec.AuthTokenRef.Name
	}

	if tunnel.Status.AuthTokenRef != nil {
		return tunnel.Status.AuthTokenRef.Name
	}
	return ""
}

// updateService updates the service with the IP address of the tunnel server
func (c *Controller) updateService(tunnel *inletsv1alpha1.Tunnel, ip string) error {

	if !ownsService(tunnel) {
		return nil
	}

	get := metav1.GetOptions{}
	res, err := c.kubeclientset.CoreV1().
		Services(tunnel.Namespace).
		Get(context.Background(), tunnel.Spec.ServiceRef.Name, get)
	if err != nil {
		return err
	}

	// Update Spec.ExternalIPs
	copy := res.DeepCopy()
	if ip == "" {
		ips := []string{}
		for _, v := range copy.Spec.ExternalIPs {
			if v != tunnel.Status.HostIP {
				ips = append(ips, v)
			}
		}
		copy.Spec.ExternalIPs = ips
	} else {
		copy.Spec.ExternalIPs = append(copy.Spec.ExternalIPs, ip)
	}

	res, err = c.kubeclientset.CoreV1().
		Services(tunnel.Namespace).
		Update(context.Background(), copy, metav1.UpdateOptions{})

	if err != nil {
		return err
	}

	// Update Status.LoadBalancer.Ingress
	copy = res.DeepCopy()
	copy.Status.LoadBalancer.Ingress = make([]corev1.LoadBalancerIngress, len(copy.Spec.ExternalIPs))
	for i, ip := range copy.Spec.ExternalIPs {
		copy.Status.LoadBalancer.Ingress[i] = corev1.LoadBalancerIngress{IP: ip}
	}

	if _, err = c.kubeclientset.CoreV1().
		Services(tunnel.Namespace).
		UpdateStatus(context.Background(), copy, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

// ownsService returns true if the tunnel owns the service
// in which case, the service's IP will also be updated.
//
// AE: Required for IPVS mode, so users can create a Tunnel Custom Resource via YAML
// without IPVS setting an IP and confusing itself.
func ownsService(tunnel *inletsv1alpha1.Tunnel) bool {
	svcs := tunnel.GetOwnerReferences()
	for _, svc := range svcs {
		if svc.Kind == "Service" {
			return true
		}
	}
	return false
}

func (c *Controller) updateTunnelProvisioningStatus(tunnel *inletsv1alpha1.Tunnel, status, id, ip string) (*inletsv1alpha1.Tunnel, error) {
	if tunnel.Spec.ServiceRef == nil {
		return tunnel, fmt.Errorf("tunnel %s.%s has no service reference", tunnel.Name, tunnel.Namespace)
	}

	klog.Infof("Status (%s.%s): %s, ID: %s, IP: %s\n", tunnel.Spec.ServiceRef.Name, tunnel.Namespace, status, id, ip)

	tunnelCopy := tunnel.DeepCopy()
	tunnelCopy.Status.HostStatus = status
	tunnelCopy.Status.HostID = id
	tunnelCopy.Status.HostIP = ip

	tunnel, err := c.operatorclientset.OperatorV1alpha1().
		Tunnels(tunnel.Namespace).
		UpdateStatus(context.Background(), tunnelCopy, metav1.UpdateOptions{})

	return tunnel, err
}

// enqueueTunnel takes a Tunnel resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Tunnel.
func (c *Controller) enqueueTunnel(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Tunnel resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Tunnel resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}

	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Tunnel, we should not do anything more
		// with it.
		if ownerRef.Kind != "Tunnel" {
			return
		}

		tunnel, err := c.tunnelsLister.Tunnels(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of tunnel '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueTunnel(tunnel)
		return
	}
}

func manageService(controller Controller, service corev1.Service) bool {
	annotations := service.Annotations

	// If the service has the annotation, use that value
	v, ok := annotations["operator.inlets.dev/manage"]
	if ok {
		valueBool, _ := strconv.ParseBool(v)
		return valueBool || v == "1"
	}

	// Else only manage if AnnotationOnly is false
	return controller.infraConfig.AnnotatedOnly == false
}

func getPortsString(service *corev1.Service) string {
	if service == nil {
		return ""
	}

	ports := ""
	for _, p := range service.Spec.Ports {
		ports = ports + fmt.Sprintf("%d,", p.Port)
	}

	return strings.TrimRight(ports, ",")
}
