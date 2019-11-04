package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	password "github.com/sethvargo/go-password/password"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

	provision "github.com/inlets/inlets-operator/pkg/provision"

	inletsv1alpha1 "github.com/inlets/inlets-operator/pkg/apis/inletsoperator/v1alpha1"
	clientset "github.com/inlets/inlets-operator/pkg/generated/clientset/versioned"
	samplescheme "github.com/inlets/inlets-operator/pkg/generated/clientset/versioned/scheme"
	informers "github.com/inlets/inlets-operator/pkg/generated/informers/externalversions/inletsoperator/v1alpha1"
	listers "github.com/inlets/inlets-operator/pkg/generated/listers/inletsoperator/v1alpha1"
)

const controllerAgentName = "sample-controller"
const inletsControlPort = 8080
const inletsProControlPort = 8123

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

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	operatorClient clientset.Interface,
	deploymentInformer appsinformers.DeploymentInformer,
	tunnelInformer informers.TunnelInformer,
	serviceInformer coreinformers.ServiceInformer,
	infra *InfraConfig) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
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
		AddFunc: controller.enqueueTunnel,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueTunnel(new)
		},
		DeleteFunc: func(old interface{}) {
			r, ok := checkCustomResourceType(old)
			if ok {
				if len(r.Status.HostID) > 0 {
					var provisioner provision.Provisioner

					switch controller.infraConfig.Provider {
					case "digitalocean":
						provisioner, _ = provision.NewDigitalOceanProvisioner(controller.infraConfig.GetAccessKey())
						break
					case "packet":
						provisioner, _ = provision.NewPacketProvisioner(controller.infraConfig.GetAccessKey())
						break
					case "scaleway":
						provisioner, _ = provision.NewScalewayProvisioner(controller.infraConfig.GetAccessKey(), controller.infraConfig.GetSecretKey(), controller.infraConfig.OrganizationID, controller.infraConfig.Region)
						break
					}

					if provisioner != nil {
						log.Printf("Deleting exit-node for %s: %s, ip: %s\n", r.Spec.ServiceName, r.Status.HostID, r.Status.HostIP)
						err := provisioner.Delete(r.Status.HostID)
						if err != nil {
							log.Println(err)
						}
					}
				}
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
		AddFunc: controller.enqueueService,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueService(new)
		},
	})

	return controller
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
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	service, _ := c.serviceLister.Services(namespace).Get(name)

	if service != nil {
		if service.Spec.Type == "LoadBalancer" {
			tunnels := c.operatorclientset.InletsoperatorV1alpha1().
				Tunnels(service.ObjectMeta.Namespace)

			ops := metav1.GetOptions{}
			name := service.Name + "-tunnel"
			found, err := tunnels.Get(name, ops)

			if errors.IsNotFound(err) {
				if manageService(*service) {
					pwdRes, pwdErr := password.Generate(64, 10, 0, false, true)
					if pwdErr != nil {
						log.Fatalf("Error generating password for inlets server %s", pwdErr.Error())
					}

					log.Printf("Creating tunnel for %s.%s\n", name, namespace)
					tunnel := &inletsv1alpha1.Tunnel{
						Spec: inletsv1alpha1.TunnelSpec{
							ServiceName: service.Name,
							AuthToken:   pwdRes,
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: service.ObjectMeta.Namespace,
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(service, schema.GroupVersionKind{
									Group:   inletsv1alpha1.SchemeGroupVersion.Group,
									Version: inletsv1alpha1.SchemeGroupVersion.Version,
									Kind:    "Tunnel",
								}),
							},
						},
					}

					_, err := tunnels.Create(tunnel)

					if err != nil {
						log.Printf("Error creating tunnel: %s", err.Error())
					}
				}
			} else {
				log.Printf("Tunnel exists: %s\n", found.Name)

				if manageService(*service) == false {
					log.Printf("Removing tunnel: %s\n", found.Name)

					err := tunnels.Delete(found.Name, &metav1.DeleteOptions{})

					if err != nil {
						log.Printf("Error deleting tunnel: %s", err.Error())
					}
				}
			}
		}
	}

	// Get the Tunnel resource with this namespace/name
	tunnel, err := c.tunnelsLister.Tunnels(namespace).Get(name)
	if err != nil {
		// The Tunnel resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			// utilruntime.HandleError(fmt.Errorf("tunnel '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	switch tunnel.Status.HostStatus {
	case "":

		var id string

		start := time.Now()
		if c.infraConfig.Provider == "packet" {

			userData := makeUserdata(tunnel.Spec.AuthToken, c.infraConfig.UsePro(), tunnel.Spec.ServiceName)

			provisioner, _ := provision.NewPacketProvisioner(c.infraConfig.GetAccessKey())

			res, err := provisioner.Provision(provision.BasicHost{
				Name:     tunnel.Name,
				OS:       "ubuntu_16_04",
				Plan:     "t1.small.x86",
				Region:   c.infraConfig.Region,
				UserData: userData,
				Additional: map[string]string{
					"project_id": c.infraConfig.ProjectID,
				},
			})

			if err != nil {
				return err
			}
			id = res.ID

		} else if c.infraConfig.Provider == "digitalocean" {

			provisioner, _ := provision.NewDigitalOceanProvisioner(c.infraConfig.GetAccessKey())

			userData := makeUserdata(tunnel.Spec.AuthToken, c.infraConfig.UsePro(), tunnel.Spec.ServiceName)

			res, err := provisioner.Provision(provision.BasicHost{
				Name:       tunnel.Name,
				OS:         "ubuntu-16-04-x64",
				Plan:       "512mb",
				Region:     c.infraConfig.Region,
				UserData:   userData,
				Additional: map[string]string{},
			})

			if err != nil {
				return err
			}

			id = res.ID
		} else if c.infraConfig.Provider == "scaleway" {
			provisioner, _ := provision.NewScalewayProvisioner(c.infraConfig.GetAccessKey(), c.infraConfig.GetSecretKey(), c.infraConfig.OrganizationID, c.infraConfig.Region)

			userData := makeUserdata(tunnel.Spec.AuthToken, c.infraConfig.UsePro(), tunnel.Spec.ServiceName)

			res, err := provisioner.Provision(provision.BasicHost{
				Name:       tunnel.Name,
				OS:         "ubuntu-bionic",
				Plan:       "DEV1-S",
				Region:     c.infraConfig.Region,
				UserData:   userData,
				Additional: map[string]string{},
			})

			if err != nil {
				return err
			}
			id = res.ID
		}

		log.Printf("Provisioning call took: %fs\n", time.Since(start).Seconds())

		if err != nil {
			err = c.updateTunnelProvisioningStatus(tunnel, "error", "", "")
			if err != nil {
				return err
			}
		} else {
			err = c.updateTunnelProvisioningStatus(tunnel, "provisioning", id, "")
		}

		if err != nil {
			return fmt.Errorf("tunnel update error %s", err)
		}

		break

	case "provisioning":

		if c.infraConfig.Provider == "packet" {

			provisioner, _ := provision.NewPacketProvisioner(c.infraConfig.GetAccessKey())
			host, err := provisioner.Status(tunnel.Status.HostID)

			if err != nil {
				fmt.Println(err)
			}

			if host.Status == provision.ActiveStatus {
				log.Printf("Device %s is now active\n", tunnel.Spec.ServiceName)

				err := c.updateTunnelProvisioningStatus(tunnel, provision.ActiveStatus, host.ID, host.IP)
				if err != nil {
					log.Printf("Error updating tunnel status: %s, %s", tunnel.Name, err.Error())
				}

				err = c.updateService(tunnel, host.IP)
				if err != nil {
					log.Printf("Error updating service: %s, %s", tunnel.Spec.ServiceName, err.Error())
					return fmt.Errorf("tunnel update error %s", err)
				}

			} else {
				log.Printf("Still provisioning: %s\n", tunnel.Name)
			}

		} else if c.infraConfig.Provider == "digitalocean" {

			provisioner, _ := provision.NewDigitalOceanProvisioner(c.infraConfig.GetAccessKey())
			host, err := provisioner.Status(tunnel.Status.HostID)

			if err != nil {
				return err
			}
			if host.Status == provision.ActiveStatus {

				if host.IP != "" {
					err := c.updateTunnelProvisioningStatus(tunnel, provision.ActiveStatus, host.ID, host.IP)
					if err != nil {
						return err
					}

					err = c.updateService(tunnel, host.IP)
					if err != nil {
						log.Printf("Error updating service: %s, %s", tunnel.Spec.ServiceName, err.Error())
						return fmt.Errorf("tunnel update error %s", err)
					}
				}
			}
		} else if c.infraConfig.Provider == "scaleway" {
			provisioner, _ := provision.NewScalewayProvisioner(c.infraConfig.GetAccessKey(), c.infraConfig.GetSecretKey(), c.infraConfig.OrganizationID, c.infraConfig.Region)
			host, err := provisioner.Status(tunnel.Status.HostID)

			if err != nil {
				return err
			}
			if host.Status == provision.ActiveStatus {

				if host.IP != "" {
					err := c.updateTunnelProvisioningStatus(tunnel, provision.ActiveStatus, host.ID, host.IP)
					if err != nil {
						return err
					}

					err = c.updateService(tunnel, host.IP)
					if err != nil {
						log.Printf("Error updating service: %s, %s", tunnel.Spec.ServiceName, err.Error())
						return fmt.Errorf("tunnel update error %s", err)
					}
				}
			}
		}

		break
	case provision.ActiveStatus:
		if tunnel.Spec.ClientDeploymentRef == nil {
			get := metav1.GetOptions{}
			service, getServiceErr := c.kubeclientset.CoreV1().Services(tunnel.Namespace).Get(tunnel.Spec.ServiceName, get)

			if getServiceErr != nil {
				return getServiceErr
			}

			firstPort := int32(80)

			for _, port := range service.Spec.Ports {
				if port.Name == "http" {
					firstPort = port.Port
					break
				}
			}

			ports := getPortsString(service)

			client := makeClient(tunnel, firstPort, c.infraConfig.GetInletsClientImage(), c.infraConfig.UsePro(), ports, c.infraConfig.ProConfig.License)
			deployment, createDeployErr := c.kubeclientset.AppsV1().
				Deployments(tunnel.Namespace).
				Create(client)

			if createDeployErr != nil {
				log.Println(createDeployErr)
			}

			tunnel.Spec.ClientDeploymentRef = &metav1.ObjectMeta{
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
			}

			_, updateErr := c.operatorclientset.InletsoperatorV1alpha1().
				Tunnels(tunnel.Namespace).
				Update(tunnel)

			if updateErr != nil {
				log.Println(updateErr)
			}

			if updateErr != nil {
				return fmt.Errorf("tunnel update error %s", updateErr)
			}

		}

		break
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. THis could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	c.recorder.Event(tunnel, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func makeClient(tunnel *inletsv1alpha1.Tunnel, targetPort int32, clientImage string, usePro bool, ports, license string) *appsv1.Deployment {
	replicas := int32(1)
	name := tunnel.Name + "-client"
	var container corev1.Container

	if !usePro {
		container = corev1.Container{
			Name:            "client",
			Image:           clientImage,
			Command:         []string{"inlets"},
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				"client",
				"--upstream=" + fmt.Sprintf("http://%s:%d", tunnel.Spec.ServiceName, targetPort),
				"--remote=" + fmt.Sprintf("ws://%s:%d", tunnel.Status.HostIP, inletsControlPort),
				"--token=" + tunnel.Spec.AuthToken,
			},
		}
	} else {
		container = corev1.Container{
			Name:            "client",
			Image:           clientImage,
			Command:         []string{"inlets-pro"},
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				"client",
				"--connect=" + fmt.Sprintf("wss://%s:%d/connect", tunnel.Status.HostIP, inletsProControlPort),
				"--token=" + tunnel.Spec.AuthToken,
				"--tcp-ports=" + ports,
				"--license=" + license,
			},
		}
	}

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: tunnel.Namespace,
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
				},
			},
		},
	}

	return &deployment
}

func (c *Controller) updateService(tunnel *inletsv1alpha1.Tunnel, ip string) error {

	get := metav1.GetOptions{}
	res, err := c.kubeclientset.CoreV1().Services(tunnel.Namespace).Get(tunnel.Spec.ServiceName, get)
	if err != nil {
		return err
	}

	copy := res.DeepCopy()
	// copy.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{
	// 	corev1.LoadBalancerIngress{IP: ip},
	// }
	copy.Spec.ExternalIPs = []string{ip}

	_, err = c.kubeclientset.CoreV1().Services(tunnel.Namespace).Update(copy)
	return err
}

func (c *Controller) updateTunnelProvisioningStatus(tunnel *inletsv1alpha1.Tunnel, status, id, ip string) error {
	log.Printf("Status (%s): %s, ID: %s, IP: %s\n", tunnel.Spec.ServiceName, status, id, ip)

	tunnelCopy := tunnel.DeepCopy()
	tunnelCopy.Status.HostStatus = status
	tunnelCopy.Status.HostID = id
	tunnelCopy.Status.HostIP = ip

	_, err := c.operatorclientset.InletsoperatorV1alpha1().Tunnels(tunnel.Namespace).Update(tunnelCopy)
	return err
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

func makeUserdata(authToken string, usePro bool, remoteTCP string) string {
	if !usePro {
		controlPort := fmt.Sprintf("%d", inletsControlPort)

		return `#!/bin/bash
export AUTHTOKEN="` + authToken + `"
export CONTROLPORT="` + controlPort + `"
curl -sLS https://get.inlets.dev | sh
curl -sLO https://raw.githubusercontent.com/inlets/inlets/master/hack/inlets-operator.service  && \
	mv inlets-operator.service /etc/systemd/system/inlets.service && \
	echo "AUTHTOKEN=$AUTHTOKEN" > /etc/default/inlets && \
	echo "CONTROLPORT=$CONTROLPORT" >> /etc/default/inlets && \
	systemctl start inlets && \
	systemctl enable inlets`
	}

	return `#!/bin/bash
export AUTHTOKEN="` + authToken + `"
export REMOTETCP="` + remoteTCP + `"
export IP=$(curl -sfSL https://ifconfig.co)
curl -SLsf https://github.com/inlets/inlets-pro-pkg/releases/download/0.4.0/inlets-pro-linux > inlets-pro-linux && \
chmod +x ./inlets-pro-linux  && \
mv ./inlets-pro-linux /usr/local/bin/inlets-pro
curl -sLO https://raw.githubusercontent.com/inlets/inlets/master/hack/inlets-pro.service  && \
	mv inlets-pro.service /etc/systemd/system/inlets-pro.service && \
	echo "AUTHTOKEN=$AUTHTOKEN" >> /etc/default/inlets-pro && \
	echo "REMOTETCP=$REMOTETCP" >> /etc/default/inlets-pro && \
	echo "IP=$IP" >> /etc/default/inlets-pro && \
	systemctl start inlets-pro && \
	systemctl enable inlets-pro`
}

func manageService(service corev1.Service) bool {
	annotations := service.Annotations
	if v, ok := annotations["dev.inlets.manage"]; ok && v == "false" {
		return false
	}
	return true
}

func getPortsString(service *corev1.Service) string {
	ports := ""
	for _, p := range service.Spec.Ports {
		ports = ports + fmt.Sprintf("%d,", p.Port)
	}
	return strings.TrimRight(ports, ",")
}
