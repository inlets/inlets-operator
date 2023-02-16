package provision

import (
	"fmt"
	"log"
	"strings"
	"time"

	instance "github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

// ScalewayProvisioner provision a VM on scaleway.com
type ScalewayProvisioner struct {
	instanceAPI *instance.API
}

// NewScalewayProvisioner with an accessKey and secretKey
func NewScalewayProvisioner(accessKey, secretKey, organizationID, region string) (*ScalewayProvisioner, error) {
	if region == "" {
		region = "fr-par-1"
	}

	zone, err := scw.ParseZone(region)
	if err != nil {
		return nil, err
	}

	client, err := scw.NewClient(
		scw.WithAuth(accessKey, secretKey),
		scw.WithDefaultOrganizationID(organizationID),
		scw.WithDefaultZone(zone),
	)
	if err != nil {
		return nil, err
	}

	return &ScalewayProvisioner{
		instanceAPI: instance.NewAPI(client),
	}, nil
}

// Provision creates a new scaleway instance from the BasicHost type
// To provision the instance we first create the server, then set its user-data and power it on
func (p *ScalewayProvisioner) Provision(host BasicHost) (*ProvisionedHost, error) {
	log.Printf("Provisioning host with Scaleway\n")

	if host.OS == "" {
		host.OS = "ubuntu-bionic"
	}

	if host.Plan == "" {
		host.Plan = "DEV1-S"
	}

	res, err := p.instanceAPI.CreateServer(&instance.CreateServerRequest{
		Name:           host.Name,
		CommercialType: host.Plan,
		Image:          host.OS,
		Tags:           []string{"inlets"},
		// DynamicIPRequired is mandatory to get a public IP
		// otherwise scaleway doesn't attach a public IP to the instance
		DynamicIPRequired: scw.BoolPtr(true),
	})

	if err != nil {
		return nil, err
	}

	server := res.Server

	if err := p.instanceAPI.SetServerUserData(&instance.SetServerUserDataRequest{
		ServerID: server.ID,
		Key:      "cloud-init",
		Content:  strings.NewReader(host.UserData),
	}); err != nil {
		return nil, err
	}

	if _, err = p.instanceAPI.ServerAction(&instance.ServerActionRequest{
		ServerID: server.ID,
		Action:   instance.ServerActionPoweron,
	}); err != nil {
		return nil, err
	}

	return serverToProvisionedHost(server), nil

}

// Status returns the status of the scaleway instance
func (p *ScalewayProvisioner) Status(id string) (*ProvisionedHost, error) {
	res, err := p.instanceAPI.GetServer(&instance.GetServerRequest{
		ServerID: id,
	})

	if err != nil {
		return nil, err
	}

	return serverToProvisionedHost(res.Server), nil
}

// Delete deletes the provisionned instance by ID
// We should first poweroff the instance,
// otherwise we'll have: http error 400 Bad Request: instance should be powered off.
// Then we have to delete the server and attached volumes
func (p *ScalewayProvisioner) Delete(request HostDeleteRequest) error {
	var id string
	var err error

	if len(request.ID) > 0 {
		id = request.ID
	} else {
		id, err = p.lookupID(request)
		if err != nil {
			return err
		}
	}

	server, err := p.instanceAPI.GetServer(&instance.GetServerRequest{
		ServerID: id,
	})

	if err != nil {
		return err
	}

	timeout := time.Minute * 5
	if err = p.instanceAPI.ServerActionAndWait(&instance.ServerActionAndWaitRequest{
		ServerID: id,
		Action:   instance.ServerActionPoweroff,
		Timeout:  &timeout,
	}); err != nil {
		return err
	}

	if err = p.instanceAPI.DeleteServer(&instance.DeleteServerRequest{
		ServerID: id,
	}); err != nil {
		return err
	}

	for _, volume := range server.Server.Volumes {
		if err := p.instanceAPI.DeleteVolume(&instance.DeleteVolumeRequest{
			VolumeID: volume.ID,
		}); err != nil {
			return err
		}
	}

	return nil
}

// List returns a list of exit nodes
func (p *ScalewayProvisioner) List(filter ListFilter) ([]*ProvisionedHost, error) {
	var inlets []*ProvisionedHost
	page := int32(1)
	perPage := uint32(20)
	for {
		servers, err := p.instanceAPI.ListServers(&instance.ListServersRequest{Page: &page, PerPage: &perPage})
		if err != nil {
			return inlets, err
		}
		for _, server := range servers.Servers {
			for _, t := range server.Tags {
				if t == filter.Filter {
					host := &ProvisionedHost{
						IP:     server.PublicIP.Address.String(),
						ID:     server.ID,
						Status: server.State.String(),
					}
					inlets = append(inlets, host)
				}
			}
		}
		if len(servers.Servers) < int(perPage) {
			break
		}
		page = page + 1
	}
	return inlets, nil
}

func (p *ScalewayProvisioner) lookupID(request HostDeleteRequest) (string, error) {
	inlets, err := p.List(ListFilter{Filter: "inlets"})
	if err != nil {
		return "", err
	}
	for _, inlet := range inlets {
		if inlet.IP == request.IP {
			return inlet.ID, nil
		}
	}

	return "", fmt.Errorf("no host with ip: %s", request.IP)
}

func serverToProvisionedHost(server *instance.Server) *ProvisionedHost {
	var ip = ""
	if server.PublicIP != nil {
		ip = server.PublicIP.Address.String()
	}

	state := server.State.String()
	if server.State.String() == "running" {
		state = ActiveStatus
	}

	return &ProvisionedHost{
		ID:     server.ID,
		IP:     ip,
		Status: state,
	}
}
