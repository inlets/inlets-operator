package ovh

import (
	"context"
	"fmt"
	"time"

	"github.com/ovh/go-ovh/ovh"

	"github.com/pkg/errors"
)

type OVHcloud struct {
	Client      *ovh.Client
	ServiceName string
	Region      string
}

// SSHKeyCreateOptions defines the configurable options to create a SSHKey
type SSHKeyCreateOptions struct {
	Name      string `json:"name"`
	PublicKey string `json:"publicKey"` //nolint:tagliatelle
}

// SSHKey describes a OVH ssh key object
type SSHKey struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Regions     []string `json:"regions"`
	FingerPrint string   `json:"fingerPrint"` //nolint:tagliatelle
	PublicKey   string   `json:"publicKey"`   //nolint:tagliatelle
}

func NewOVHDefaultClient(region, serviceName string) (*OVHcloud, error) {
	client, err := ovh.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	return &OVHcloud{
		Client:      client,
		ServiceName: serviceName,
		Region:      region,
	}, nil
}

// NewOVHClient creates an OVHcloud Client with the parameters
func NewOVHClient(endpoint, appKey, appSecret, consumerKey, region, serviceName string) (*OVHcloud, error) {
	client, err := ovh.NewClient(endpoint, appKey, appSecret, consumerKey)
	if err != nil {
		return nil, err
	}

	return &OVHcloud{
		Client:      client,
		ServiceName: serviceName,
		Region:      region,
	}, nil
}

// CreateSSHKey creates an SSHKey with the given SSHKeyCreateOptions
func (o *OVHcloud) CreateSSHKey(ctx context.Context, createOpts SSHKeyCreateOptions) (*SSHKey, error) {
	resp := SSHKey{}
	err := o.Client.PostWithContext(ctx, fmt.Sprintf("/cloud/project/%s/sshkey", o.ServiceName), createOpts, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, err
}

// ListSSHKeys returns a slice of SSHKey
func (o *OVHcloud) ListSSHKeys(ctx context.Context) ([]SSHKey, error) {
	url := fmt.Sprintf("/cloud/project/%s/sshkey", o.ServiceName)
	var keys []SSHKey
	err := o.Client.GetWithContext(ctx, url, &keys)
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// DeleteSSHKey deletes a SSHKey
func (o *OVHcloud) DeleteSSHKey(ctx context.Context, id string) error {
	url := fmt.Sprintf("/cloud/project/%s/sshkey/%s", o.ServiceName, id)
	err := o.Client.DeleteWithContext(ctx, url, nil)
	if err != nil {
		return err
	}
	return nil
}

// VolumeType describes the different types of available OVH volumes
type VolumeType string

const (
	VolumeClassic       VolumeType = "classic"
	VolumeHighSpeed     VolumeType = "high-speed"
	VolumeHighSpeedGen2 VolumeType = "high-speed-gen2"
)

// VolumeStatus descibes the volume states
type VolumeStatus string

const (
	VolumeAttaching VolumeStatus = "attaching"
	VolumeCreating  VolumeStatus = "creating"
	VolumeAvailable VolumeStatus = "available"
	VolumeInUse     VolumeStatus = "in-use"
)

// Volume describes a OVH volume object
type Volume struct {
	ID           string       `json:"id"`
	AttachedTo   []string     `json:"attachedTo"`   //nolint:tagliatelle
	CreationDate time.Time    `json:"creationDate"` //nolint:tagliatelle
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Size         int          `json:"size"`
	Status       VolumeStatus `json:"status"`
	Region       string       `json:"region"`
	Bootable     bool         `json:"bootable"`
	PlanCode     string       `json:"planCode"` //nolint:tagliatelle
	Type         VolumeType   `json:"type"`
}

// VolumeCreateOptions defines the configurable options to create a Volume
type VolumeCreateOptions struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Size        int        `json:"size"`
	Region      string     `json:"region"`
	Type        VolumeType `json:"type"`
}

// VolumeAttachOptions defines the configurable options to attach a Volume to an Instance
type VolumeAttachOptions struct {
	InstanceID string `json:"instanceId"` //nolint:tagliatelle
}

// VolumeDetachOptions defines the configurable options to detach a Volume from an Instance
type VolumeDetachOptions VolumeAttachOptions

// CreateVolume creates a Volume
func (o *OVHcloud) CreateVolume(ctx context.Context, createOpts VolumeCreateOptions) (*Volume, error) {
	resp := Volume{}
	err := o.Client.PostWithContext(ctx, fmt.Sprintf("/cloud/project/%s/volume", o.ServiceName), createOpts, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, err
}

// ListVolumes returns a slice of Volume
func (o *OVHcloud) ListVolumes(ctx context.Context) ([]Volume, error) {
	var volumes []Volume
	err := o.Client.GetWithContext(ctx, fmt.Sprintf("/cloud/project/%s/volume", o.ServiceName), &volumes)
	if err != nil {
		return nil, err
	}
	return volumes, err
}

// GetVolume returns a Volume
func (o *OVHcloud) GetVolume(ctx context.Context, id string) (*Volume, error) {
	url := fmt.Sprintf("/cloud/project/%s/volume/%s", o.ServiceName, id)
	resp := Volume{}
	err := o.Client.GetWithContext(ctx, url, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteVolume deletes a Volume
func (o *OVHcloud) DeleteVolume(ctx context.Context, id string) error {
	url := fmt.Sprintf("/cloud/project/%s/volume/%s", o.ServiceName, id)
	err := o.Client.DeleteWithContext(ctx, url, nil)
	if err != nil {
		return err
	}
	return nil
}

// DetachVolume attaches a Volume from a Instance defined int the VolumeAttachOptions
func (o *OVHcloud) AttachVolume(ctx context.Context, id string, options *VolumeAttachOptions) (*Volume, error) {
	resp := Volume{}
	err := o.Client.PostWithContext(ctx, fmt.Sprintf("/cloud/project/%s/volume/%s/attach", o.ServiceName, id), options, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, err
}

// DetachVolume detaches a Volume from a Instance defined int the VolumeDetachOptions
func (o *OVHcloud) DetachVolume(ctx context.Context, id string, options *VolumeDetachOptions) (*Volume, error) {
	resp := Volume{}
	err := o.Client.PostWithContext(ctx, fmt.Sprintf("/cloud/project/%s/volume/%s/detach", o.ServiceName, id), options, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, err
}

// Image describes an OVH image
type Image struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Region       string    `json:"region"`
	Visibility   string    `json:"visibility"`
	Type         string    `json:"type"`
	MinDisk      int       `json:"minDisk"` //nolint:tagliatelle
	MinRAM       int       `json:"minRam"`  //nolint:tagliatelle
	Size         float64   `json:"size"`
	CreationDate time.Time `json:"creationDate"` //nolint:tagliatelle
	Status       string    `json:"status"`
	User         string    `json:"user"`
	FlavorType   string    `json:"flavorType"` //nolint:tagliatelle
	Tags         []string  `json:"tags"`
	PlanCode     string    `json:"planCode"` //nolint:tagliatelle
}

// GetImage returns the available Image in a Region
func (o *OVHcloud) GetImage(ctx context.Context, name, region string) (*Image, error) {
	url := fmt.Sprintf("/cloud/project/%s/image", o.ServiceName)
	var images []Image
	err := o.Client.GetWithContext(ctx, url, &images)
	if err != nil {
		return nil, err
	}
	for _, image := range images {
		if image.Region == region && image.Name == name {
			return &image, nil
		}
	}
	return nil, errors.Errorf("image: %s in Region: %s not found", name, region)
}

// Flavor describes the OVH flavors
type Flavor struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Region            string `json:"region"`
	RAM               int    `json:"ram"`
	Disk              int    `json:"disk"`
	Vcpus             int    `json:"vcpus"`
	Type              string `json:"type"`
	OsType            string `json:"osType"`            //nolint:tagliatelle
	InboundBandwidth  int    `json:"inboundBandwidth"`  //nolint:tagliatelle
	OutboundBandwidth int    `json:"outboundBandwidth"` //nolint:tagliatelle
	Available         bool   `json:"available"`
	PlanCodes         struct {
		Monthly string `json:"monthly"`
		Hourly  string `json:"hourly"`
	} `json:"planCodes"` //nolint:tagliatelle
	Capabilities []struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	} `json:"capabilities"`
	Quota int `json:"quota"`
}

// GetFlavor returns the available Flavor in a Region
func (o *OVHcloud) GetFlavor(ctx context.Context, name, region string) (*Flavor, error) {
	url := fmt.Sprintf("/cloud/project/%s/flavor", o.ServiceName)
	var flavors []Flavor
	err := o.Client.GetWithContext(ctx, url, &flavors)
	if err != nil {
		return nil, err
	}
	for _, flavor := range flavors {
		if flavor.Region == region && flavor.Name == name {
			return &flavor, nil
		}
	}
	return nil, errors.Errorf("flavor: %s in Region: %s not found", name, region)
}

// IP describes the properties of an OVH ip object
type IP struct {
	IP        string `json:"ip"`
	Type      string `json:"type"`
	Version   int    `json:"version"`
	NetworkID string `json:"networkId"` //nolint:tagliatelle
	GatewayIP string `json:"gatewayIp"` //nolint:tagliatelle
}

// Instance describe the properties of an OVH instance
type Instance struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	IPAddresses    []IP           `json:"ipAddresses"` //nolint:tagliatelle
	FlavorID       string         `json:"flavorId"`    //nolint:tagliatelle
	ImageID        string         `json:"imageId"`     //nolint:tagliatelle
	SSHKeyID       string         `json:"sshKeyId"`    //nolint:tagliatelle
	Created        time.Time      `json:"created"`
	Region         string         `json:"region"`
	MonthlyBilling string         `json:"monthlyBilling"` //nolint:tagliatelle
	Status         InstanceStatus `json:"status"`
	PlanCode       string         `json:"planCode"`     //nolint:tagliatelle
	OperationIds   []string       `json:"operationIds"` //nolint:tagliatelle
}

// InstanceStatus selection of possible instance states
type InstanceStatus string

const (
	InstanceActive   InstanceStatus = "ACTIVE"
	InstanceBuilding InstanceStatus = "BUILDING"
	InstanceDeleted  InstanceStatus = "DELETED"
	InstanceDeleting InstanceStatus = "DELETING"
	InstanceError    InstanceStatus = "ERROR"
	InstanceReboot   InstanceStatus = "REBOOT"
	InstanceStopped  InstanceStatus = "STOPPED"
	InstanceUnknown  InstanceStatus = "UNKNOWN"
	InstanceBuild    InstanceStatus = "BUILD"
	InstanceResuming InstanceStatus = "RESUMING"
	InstanceRebuild  InstanceStatus = "REBUILD"
)

// InstanceCreateOptions defines the configurable options to create a Instance
type InstanceCreateOptions struct {
	FlavorID       string `json:"flavorId"`       //nolint:tagliatelle
	ImageID        string `json:"imageId"`        //nolint:tagliatelle
	MonthlyBilling bool   `json:"monthlyBilling"` //nolint:tagliatelle
	Name           string `json:"name"`
	Region         string `json:"region"`
	SSHKeyID       string `json:"sshKeyId"` //nolint:tagliatelle
	UserData       string `json:"userData"` //nolint:tagliatelle
}

// CreateInstance creates an Instance with the given InstanceCreateOptions
func (o *OVHcloud) CreateInstance(ctx context.Context, createOpts InstanceCreateOptions) (*Instance, error) {
	resp := Instance{}
	err := o.Client.PostWithContext(ctx, fmt.Sprintf("/cloud/project/%s/instance", o.ServiceName), createOpts, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, err
}

// GetInstance returns a single Instance by id
func (o *OVHcloud) GetInstance(ctx context.Context, id string) (*Instance, error) {
	url := fmt.Sprintf("/cloud/project/%s/instance/%s", o.ServiceName, id)
	resp := Instance{}
	err := o.Client.GetWithContext(ctx, url, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListInstance returns slice of Instance
func (o *OVHcloud) ListInstance(ctx context.Context) ([]Instance, error) {
	url := fmt.Sprintf("/cloud/project/%s/instance?Region=%s", o.ServiceName, o.Region)
	var resp []Instance
	err := o.Client.GetWithContext(ctx, url, &resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// DeleteInstance deletes an Instance
func (o *OVHcloud) DeleteInstance(ctx context.Context, id string) error {
	url := fmt.Sprintf("/cloud/project/%s/instance/%s", o.ServiceName, id)
	err := o.Client.DeleteWithContext(ctx, url, nil)
	if err != nil {
		return err
	}
	return nil
}

// IPv4 get the IP4 address
func IPv4(instance *Instance) (string, error) {
	for _, ip := range instance.IPAddresses {
		if ip.Version == 4 {
			return ip.IP, nil
		}
	}
	return "", errors.Errorf("no ip4 address found for instance: %s", instance.Name)
}
