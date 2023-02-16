package provision

import (
	"context"
	"fmt"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"strconv"
)

var Status = "running"

type HetznerProvisioner struct {
	client *hcloud.Client
}

// Creates a new Hetzner provisioner using an auth token.
func NewHetznerProvisioner(authToken string) (*HetznerProvisioner, error) {
	client := hcloud.NewClient(hcloud.WithToken(authToken))
	return &HetznerProvisioner{
		client: client,
	}, nil
}

// Get status of a specific server by id.
func (p *HetznerProvisioner) Status(id string) (*ProvisionedHost, error) {
	intId, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	server, _, err := p.client.Server.GetByID(context.Background(), intId)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, fmt.Errorf("failed to find server with id %s", id)
	}

	status := ""
	ip := ""

	if server.Status == hcloud.ServerStatusRunning {
		status = ActiveStatus
		ip = server.PublicNet.IPv4.IP.String()
	} else {
		status = string(server.Status)
	}

	return &ProvisionedHost{
		IP:     ip,
		ID:     id,
		Status: status,
	}, nil
}

// Provision a new server on Hetzner cloud to use as an inlet node.
func (p *HetznerProvisioner) Provision(host BasicHost) (*ProvisionedHost, error) {
	img, _, err := p.client.Image.GetByName(context.Background(), host.OS)
	loc, _, err := p.client.Location.GetByName(context.Background(), host.Region)
	pln, _, err := p.client.ServerType.GetByName(context.Background(), host.Plan)

	if err != nil {
		return nil, err
	}

	server, _, err := p.client.Server.Create(context.Background(), hcloud.ServerCreateOpts{
		Name:             host.Name,
		ServerType:       pln,
		Image:            img,
		Location:         loc,
		UserData:         host.UserData,
		StartAfterCreate: hcloud.Bool(true),
		Labels: map[string]string{
			"managed-by": "inlets",
		},
	})

	if err != nil {
		return nil, err
	}

	return &ProvisionedHost{
		IP:     server.Server.PublicNet.IPv4.IP.String(),
		ID:     strconv.Itoa(server.Server.ID),
		Status: "creating",
	}, nil
}

// List all nodes that are managed by inlets.
func (p *HetznerProvisioner) List(filter ListFilter) ([]*ProvisionedHost, error) {
	var hosts []*ProvisionedHost
	hostList, err := p.client.Server.AllWithOpts(context.Background(), hcloud.ServerListOpts{
		// Adding a label to the VPS so that it is easier to select inlets managed servers and also
		// to tell the user that the server in question is managed by inlets.
		ListOpts: hcloud.ListOpts{
			LabelSelector: "managed-by=inlets",
		},
	})

	if err != nil {
		return nil, err
	}

	for _, instance := range hostList {
		hosts = append(hosts, &ProvisionedHost{
			IP:     instance.PublicNet.IPv4.IP.String(),
			ID:     strconv.Itoa(instance.ID),
			Status: string(instance.Status),
		})
	}

	return hosts, nil
}

// Delete a specific server from the Hetzner cloud.
func (p *HetznerProvisioner) Delete(request HostDeleteRequest) error {
	id := request.ID
	if len(id) <= 0 {
		hosts, err := p.List(ListFilter{})
		if err != nil {
			return err
		}
		for _, instance := range hosts {
			if instance.IP == request.IP {
				id = instance.ID
			}
		}
		if len(id) <= 0 {
			return fmt.Errorf("failed to find server with id with IP %s", request.IP)
		}
	}

	idAsInt, err := strconv.Atoi(id)
	if err != nil {
		return err
	}

	server, _, err := p.client.Server.GetByID(context.Background(), idAsInt)
	if err != nil {
		return err
	}

	if server == nil {
		return fmt.Errorf("failed to find server with id %s", id)
	}

	_, err = p.client.Server.Delete(context.Background(), server)
	if err != nil {
		return err
	}

	return nil
}
