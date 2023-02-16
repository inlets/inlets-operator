package provision

import (
	"context"
	"fmt"
	ovhsdk "github.com/dirien/ovh-go-sdk/pkg/sdk"
)

type OVHProvisioner struct {
	client *ovhsdk.OVHcloud
}

func NewOVHProvisioner(endpoint, appKey, appSecret, consumerKey, region, serviceName string) (*OVHProvisioner, error) {
	client, err := ovhsdk.NewOVHClient(endpoint, appKey, appSecret, consumerKey, region, serviceName)
	if err != nil {
		return nil, err
	}

	return &OVHProvisioner{
		client: client,
	}, nil
}

func (o *OVHProvisioner) Provision(host BasicHost) (*ProvisionedHost, error) {
	image, err := o.client.GetImage(context.Background(), host.OS, host.Region)
	if err != nil {
		return nil, err
	}
	flavor, err := o.client.GetFlavor(context.Background(), host.Plan, host.Region)
	if err != nil {
		return nil, err
	}

	instance, err := o.client.CreateInstance(context.Background(), ovhsdk.InstanceCreateOptions{
		Name:     host.Name,
		ImageID:  image.ID,
		Region:   host.Region,
		FlavorID: flavor.ID,
		UserData: host.UserData,
	})
	if err != nil {
		return nil, err
	}

	//ignore missing ip4 during build
	ip4, _ := ovhsdk.IPv4(instance)

	return &ProvisionedHost{
		ID:     instance.ID,
		IP:     ip4,
		Status: string(ovhsdk.InstanceBuilding),
	}, nil
}

func ovhToInletsStatus(ovhStatus ovhsdk.InstanceStatus) string {
	status := string(ovhStatus)
	if status == string(ovhsdk.InstanceActive) {
		status = ActiveStatus
	}
	return status
}

func (o *OVHProvisioner) Status(id string) (*ProvisionedHost, error) {
	instance, err := o.client.GetInstance(context.Background(), id)
	if err != nil {
		return nil, err
	}
	//ignore missing ip4 during build
	ip4, _ := ovhsdk.IPv4(instance)

	status := ovhToInletsStatus(instance.Status)
	return &ProvisionedHost{
		ID:     instance.ID,
		IP:     ip4,
		Status: status,
	}, nil
}

func (o *OVHProvisioner) lookupID(request HostDeleteRequest) (string, error) {
	instances, err := o.client.ListInstance(context.Background())
	if err != nil {
		return "", err
	}
	for _, instance := range instances {
		ip4, _ := ovhsdk.IPv4(&instance)
		if ip4 == request.IP {
			return instance.ID, nil
		}
	}
	return "", fmt.Errorf("no host with ip: %s", request.IP)
}

func (o *OVHProvisioner) Delete(request HostDeleteRequest) error {
	var id string
	var err error
	if len(request.ID) > 0 {
		id = request.ID
	} else {
		id, err = o.lookupID(request)
		if err != nil {
			return err
		}
	}
	err = o.client.DeleteInstance(context.Background(), id)
	if err != nil {
		return err
	}
	return nil
}
