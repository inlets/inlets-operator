package provision

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/linode/linodego"
	"github.com/sethvargo/go-password/password"
	"golang.org/x/oauth2"
)

type LinodeInterface interface {
	CreateStackscript(createOpts linodego.StackscriptCreateOptions) (*linodego.Stackscript, error)
	CreateInstance(instance linodego.InstanceCreateOptions) (*linodego.Instance, error)
	GetInstance(linodeID int) (*linodego.Instance, error)
	DeleteInstance(linodeID int) error
	DeleteStackscript(id int) error
}

type LinodeClient struct {
	client linodego.Client
}

func (p *LinodeClient) CreateStackscript(createOpts linodego.StackscriptCreateOptions) (*linodego.Stackscript, error) {
	return p.client.CreateStackscript(context.Background(), createOpts)
}

func (p *LinodeClient) CreateInstance(instance linodego.InstanceCreateOptions) (*linodego.Instance, error) {
	return p.client.CreateInstance(context.Background(), instance)
}

func (p *LinodeClient) GetInstance(linodeID int) (*linodego.Instance, error) {
	return p.client.GetInstance(context.Background(), linodeID)
}

func (p *LinodeClient) DeleteInstance(linodeID int) error {
	return p.client.DeleteInstance(context.Background(), linodeID)
}

func (p *LinodeClient) DeleteStackscript(id int) error {
	return p.client.DeleteStackscript(context.Background(), id)
}

type LinodeProvisioner struct {
	client        LinodeInterface
	stackscriptID int
}

func NewLinodeProvisioner(apiKey string) (*LinodeProvisioner, error) {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})

	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}

	linodeClient := linodego.NewClient(oauth2Client)
	return &LinodeProvisioner{
		client: &LinodeClient{linodeClient},
	}, nil
}

// Provision provisions a new Linode instance as an exit node
func (p *LinodeProvisioner) Provision(host BasicHost) (*ProvisionedHost, error) {

	if len(host.Name) > 32 {
		return nil, fmt.Errorf("name cannot be longer than 32 characters for Linode due to label limitations")
	}

	// Stack script is how linode does the cloud-init when provisioning a VM.
	// Stack script is the inlets user data containing inlets auth token.
	// Making stack script public will allow everyone to read the stack script
	// and hence allow them to access the inlets auth token.
	// Because of above, here the stack script is created as IsPublic: false, so no one else can read it.
	// This script will be deleted once VM instance is running.
	stackscriptOption := linodego.StackscriptCreateOptions{
		IsPublic: false, Images: []string{host.OS}, Script: host.UserData, Label: host.Name,
	}

	stackscript, err := p.client.CreateStackscript(stackscriptOption)
	if err != nil {
		return nil, err
	}
	p.stackscriptID = stackscript.ID

	// Create instance
	rootPassword, err := password.Generate(16, 4, 0, false, true)
	if err != nil {
		return nil, err
	}
	instanceOptions := linodego.InstanceCreateOptions{
		Label:         host.Name,
		StackScriptID: stackscript.ID,
		Image:         host.OS,
		Region:        host.Region,
		Type:          host.Plan,
		RootPass:      rootPassword,
		Tags:          []string{"inlets"},
	}
	instance, err := p.client.CreateInstance(instanceOptions)
	if err != nil {
		return nil, err
	}

	return &ProvisionedHost{
		IP:     "",
		ID:     fmt.Sprintf("%d", instance.ID),
		Status: string(instance.Status),
	}, nil
}

// Status checks the status of the provisioning Linode exit node
func (p *LinodeProvisioner) Status(id string) (*ProvisionedHost, error) {
	instanceId, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	instance, err := p.client.GetInstance(instanceId)
	if err != nil {
		return nil, err
	}
	var status string
	if instance.Status == linodego.InstanceRunning {
		status = ActiveStatus
	} else {
		status = string(instance.Status)
	}
	IP := ""
	if status == ActiveStatus {
		if len(instance.IPv4) > 0 {
			IP = instance.IPv4[0].String()
		}
		// If it fails to delete stack script, we will ignore and let user to delete manually.
		// It won't create security issue as this script was created as a private script.
		_ = p.client.DeleteStackscript(p.stackscriptID)
	}
	return &ProvisionedHost{
		IP:     IP,
		ID:     id,
		Status: status,
	}, nil
}

// Delete deletes the Linode exit node
func (p *LinodeProvisioner) Delete(request HostDeleteRequest) error {
	instanceId, err := strconv.Atoi(request.ID)
	if err != nil {
		return err
	}
	err = p.client.DeleteInstance(instanceId)
	return err
}
