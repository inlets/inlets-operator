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

// Status returns the status of an exit node
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

// Delete terminates an exit node
func (p *DigitalOceanProvisioner) Delete(request HostDeleteRequest) error {
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
	sid, err := strconv.Atoi(id)
	if err != nil {
		return err
	}
	_, err = p.client.Droplets.Delete(context.Background(), sid)
	return err
}

// List returns a list of exit nodes
func (p *DigitalOceanProvisioner) List(filter ListFilter) ([]*ProvisionedHost, error) {
	var inlets []*ProvisionedHost
	opt := &godo.ListOptions{}
	for {
		droplets, resp, err := p.client.Droplets.ListByTag(context.Background(), filter.Filter, opt)
		if err != nil {
			return inlets, err
		}
		for _, droplet := range droplets {
			publicIP, err := droplet.PublicIPv4()
			if err != nil {
				return inlets, err
			}
			host := &ProvisionedHost{
				IP: publicIP,
				ID: fmt.Sprintf("%d", droplet.ID),
			}
			inlets = append(inlets, host)
		}
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}
		page, err := resp.Links.CurrentPage()
		if err != nil {
			return inlets, err
		}
		opt.Page = page + 1
	}
	return inlets, nil
}

func (p *DigitalOceanProvisioner) lookupID(request HostDeleteRequest) (string, error) {
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

// Provision creates an exit node
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
		Tags:     []string{"inlets"},
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

// TokenSource contains an access token
type TokenSource struct {
	AccessToken string
}

// Token returns an oauth2 token
func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}
