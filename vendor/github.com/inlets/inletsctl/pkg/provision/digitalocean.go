package provision

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

// DigitalOceanProvisioner provision a VM on digitalocean.com
type DigitalOceanProvisioner struct {
	client *godo.Client
}

// NewDigitalOceanProvisioner with an accessKey
func NewDigitalOceanProvisioner(accessKey string) (*DigitalOceanProvisioner, error) {

	tokenSource := &TokenSource{
		AccessToken: accessKey,
	}
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	client := godo.NewClient(oauthClient)

	return &DigitalOceanProvisioner{
		client: client,
	}, nil
}

func (p *DigitalOceanProvisioner) Status(id string) (*ProvisionedHost, error) {
	sid, _ := strconv.Atoi(id)

	droplet, _, err := p.client.Droplets.Get(context.Background(), sid)

	if err != nil {
		return nil, err
	}

	state := droplet.Status

	ip := ""
	for _, network := range droplet.Networks.V4 {
		if network.Type == "public" {
			ip = network.IPAddress
		}
	}

	return &ProvisionedHost{
		ID:     id,
		Status: state,
		IP:     ip,
	}, nil
}

func (p *DigitalOceanProvisioner) Delete(id string) error {
	sid, _ := strconv.Atoi(id)
	_, err := p.client.Droplets.Delete(context.Background(), sid)
	return err
}

func (p *DigitalOceanProvisioner) Provision(host BasicHost) (*ProvisionedHost, error) {

	log.Printf("Provisioning host with DigitalOcean\n")

	if host.Region == "" {
		host.Region = "lon1"
	}

	createReq := &godo.DropletCreateRequest{
		Name:   host.Name,
		Region: host.Region,
		Size:   host.Plan,
		Image: godo.DropletCreateImage{
			Slug: host.OS,
		},
		UserData: host.UserData,
	}

	droplet, _, err := p.client.Droplets.Create(context.Background(), createReq)

	if err != nil {
		return nil, err
	}

	return &ProvisionedHost{
		ID: fmt.Sprintf("%d", droplet.ID),
	}, nil
}

type TokenSource struct {
	AccessToken string
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}
