package provision

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/vultr/govultr/v2"
	"golang.org/x/oauth2"
	"strconv"
	"strings"
)

const vultrHostRunning = "ok"
const exiteNodeTag = "inlets-exit-node"

type VultrProvisioner struct {
	client *govultr.Client
}

func NewVultrProvisioner(accessKey string) (*VultrProvisioner, error) {
	config := &oauth2.Config{}
	ts := config.TokenSource(context.Background(), &oauth2.Token{AccessToken: accessKey})
	vultrClient := govultr.NewClient(oauth2.NewClient(context.Background(), ts))
	return &VultrProvisioner{
		client: vultrClient,
	}, nil
}

func (v *VultrProvisioner) Provision(host BasicHost) (*ProvisionedHost, error) {
	script, err := v.client.StartupScript.Create(context.Background(), &govultr.StartupScriptReq{
		Script: base64.StdEncoding.EncodeToString([]byte(host.UserData)),
		Name:   host.Name,
		Type:   "boot",
	})
	if err != nil {
		return nil, err
	}

	os, err := strconv.Atoi(host.OS)
	if err != nil {
		return nil, err
	}

	opts := &govultr.InstanceCreateReq{
		ScriptID: script.ID,
		Region:   host.Region,
		Plan:     host.Plan,
		OsID:     os,
		Hostname: host.Name,
		Label:    host.Name,
		Tag:      exiteNodeTag,
	}

	result, err := v.client.Instance.Create(context.Background(), opts)
	if err != nil {
		return nil, err
	}

	return &ProvisionedHost{
		IP:     result.MainIP,
		ID:     result.ID,
		Status: result.ServerStatus,
	}, nil
}

func (v *VultrProvisioner) Status(id string) (*ProvisionedHost, error) {
	server, err := v.client.Instance.Get(context.Background(), id)
	if err != nil {
		return nil, err
	}

	status := server.ServerStatus
	if status == vultrHostRunning {
		status = ActiveStatus
	}

	return &ProvisionedHost{
		IP:     server.MainIP,
		ID:     server.ID,
		Status: status,
	}, nil
}

func (v *VultrProvisioner) Delete(request HostDeleteRequest) error {
	var id string
	var err error
	if len(request.ID) > 0 {
		id = request.ID
	} else {
		id, err = v.lookupID(request)
		if err != nil {
			return err
		}
	}

	server, err := v.client.Instance.Get(context.Background(), id)
	if err != nil {
		return err
	}

	err = v.client.Instance.Delete(context.Background(), id)
	if err != nil {
		return err
	}

	scripts, _, err := v.client.StartupScript.List(context.Background(), nil)
	for _, s := range scripts {
		if s.Name == server.Label {
			_ = v.client.StartupScript.Delete(context.Background(), s.ID)
			break
		}
	}

	return nil
}

// List returns a list of exit nodes
func (v *VultrProvisioner) List(filter ListFilter) ([]*ProvisionedHost, error) {
	servers, _, err := v.client.Instance.List(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	var inlets []*ProvisionedHost
	for _, server := range servers {
		if server.Tag == filter.Filter {
			host := &ProvisionedHost{
				IP:     server.MainIP,
				ID:     server.ID,
				Status: vultrToInletsStatus(server.Status),
			}
			inlets = append(inlets, host)
		}

	}

	return inlets, nil
}

func (v *VultrProvisioner) lookupID(request HostDeleteRequest) (string, error) {

	inlets, err := v.List(ListFilter{Filter: exiteNodeTag})
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

func (v *VultrProvisioner) lookupRegion(id string) (*int, error) {
	result, err := strconv.Atoi(id)
	if err == nil {
		return &result, nil
	}

	regions, _, err := v.client.Region.List(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	for _, region := range regions {
		if strings.EqualFold(id, region.ID) {
			regionId, _ := strconv.Atoi(region.ID)
			return &regionId, nil
		}
	}

	return nil, fmt.Errorf("region '%s' not available", id)
}

func vultrToInletsStatus(vultr string) string {
	status := vultr
	if status == vultrHostRunning {
		status = ActiveStatus
	}
	return status
}
