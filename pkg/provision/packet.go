package provision

import (
	"net/http"

	"github.com/packethost/packngo"
)

// PacketProvisioner provision a host on Packet.com
type PacketProvisioner struct {
	client *packngo.Client
}

// NewPacketProvisioner with an accessKey
func NewPacketProvisioner(accessKey string) (*PacketProvisioner, error) {
	return &PacketProvisioner{
		client: packngo.NewClientWithAuth("", accessKey, http.DefaultClient),
	}, nil
}

func (p *PacketProvisioner) Status(id string) (*ProvisionedHost, error) {
	device, _, err := p.client.Devices.Get(id, nil)

	if err != nil {
		return nil, err
	}

	state := device.State

	ip := ""
	for _, network := range device.Network {
		if network.Public {
			ip = network.IpAddressCommon.Address
			break
		}
	}

	return &ProvisionedHost{
		ID:     device.ID,
		Status: state,
		IP:     ip,
	}, nil
}

func (p *PacketProvisioner) Delete(id string) error {
	_, err := p.client.Devices.Delete(id)
	return err
}

func (p *PacketProvisioner) Provision(host BasicHost) (*ProvisionedHost, error) {
	if host.Region == "" {
		host.Region = "ams1"
	}

	createReq := &packngo.DeviceCreateRequest{
		Plan:         host.Plan,
		Facility:     []string{host.Region},
		Hostname:     host.Name,
		ProjectID:    host.Additional["project_id"],
		SpotInstance: false,
		OS:           host.OS,
		BillingCycle: "hourly",
		UserData:     host.UserData,
	}

	device, _, err := p.client.Devices.Create(createReq)

	if err != nil {
		return nil, err
	}

	return &ProvisionedHost{
		ID: device.ID,
	}, nil
}
