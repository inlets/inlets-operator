// Copyright (c) Inlets Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package provision

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"strings"
	"time"
)

// CivoProvisioner creates instances on civo.com
type CivoProvisioner struct {
	APIKey string
}

// Network represents a network for civo vm instances to connect to
type Network struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Default bool   `json:"default,omitempty"`
	CIDR    string `json:"cidr,omitempty"`
	Label   string `json:"label,omitempty"`
}

// NewCivoProvisioner with an accessKey
func NewCivoProvisioner(accessKey string) (*CivoProvisioner, error) {
	return &CivoProvisioner{
		APIKey: accessKey,
	}, nil
}

// Status gets the status of the exit node
func (p *CivoProvisioner) Status(id string) (*ProvisionedHost, error) {
	host := &ProvisionedHost{}

	apiURL := fmt.Sprint("https://api.civo.com/v2/instances/", id)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return host, err
	}
	addAuth(req, p.APIKey)

	req.Header.Add("Accept", "application/json")
	instance := createdInstance{}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return host, err
	}

	var body []byte
	if res.Body != nil {
		defer res.Body.Close()
		body, _ = ioutil.ReadAll(res.Body)
	}

	if res.StatusCode != http.StatusOK {
		return host, fmt.Errorf("unexpected HTTP code: %d\n%q", res.StatusCode, string(body))
	}

	unmarshalErr := json.Unmarshal(body, &instance)
	if unmarshalErr != nil {
		return host, unmarshalErr
	}

	return &ProvisionedHost{
		ID:     instance.ID,
		IP:     instance.PublicIP,
		Status: strings.ToLower(instance.Status),
	}, nil
}

// Delete terminates the exit node
func (p *CivoProvisioner) Delete(request HostDeleteRequest) error {
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

	apiURL := fmt.Sprint("https://api.civo.com/v2/instances/", id)
	_, err = apiCall(p.APIKey, http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}
	return nil
}

// Provision creates a new exit node
func (p *CivoProvisioner) Provision(host BasicHost) (*ProvisionedHost, error) {

	log.Printf("Provisioning host with Civo\n")

	if host.Region == "" {
		host.Region = "lon1"
	}

	res, err := provisionCivoInstance(host, p.APIKey)

	if err != nil {
		return nil, err
	}

	return &ProvisionedHost{
		ID: res.ID,
	}, nil
}

// List returns a list of exit nodes
func (p *CivoProvisioner) List(filter ListFilter) ([]*ProvisionedHost, error) {
	var inlets []*ProvisionedHost
	apiURL := fmt.Sprintf("https://api.civo.com/v2/instances/?tags=%s", filter.Filter)
	body, err := apiCall(p.APIKey, http.MethodGet, apiURL, nil)
	if err != nil {
		return inlets, err
	}

	var resp apiResponse
	unmarshalErr := json.Unmarshal(body, &resp)
	if unmarshalErr != nil {
		return inlets, unmarshalErr
	}

	for _, instance := range resp.Items {
		host := &ProvisionedHost{
			IP:     instance.PublicIP,
			ID:     instance.ID,
			Status: instance.Status,
		}
		inlets = append(inlets, host)
	}
	return inlets, nil
}

func (p *CivoProvisioner) lookupID(request HostDeleteRequest) (string, error) {
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

// gets the default network for the selected region.
func getDefaultNetwork(key, region string) (*Network, error) {
	apiURL := "https://api.civo.com/v2/networks"
	values := url.Values{}
	values.Add("region", region)
	body, err := apiCall(key, http.MethodGet, apiURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}

	networks := []Network{}
	if err := json.Unmarshal(body, &networks); err != nil {
		return nil, err
	}

	for _, network := range networks {
		if network.Default {
			return &network, nil
		}
	}
	return nil, errors.New("no default network found")
}

func provisionCivoInstance(host BasicHost, key string) (createdInstance, error) {
	instance := createdInstance{}

	network, err := getDefaultNetwork(key, host.Region)
	if err != nil {
		return instance, err
	}

	apiURL := "https://api.civo.com/v2/instances"

	values := url.Values{}
	values.Add("hostname", host.Name)
	values.Add("size", host.Plan)
	values.Add("public_ip", "create")
	values.Add("template_id", host.OS)
	values.Add("initial_user", "civo")
	values.Add("script", host.UserData)
	values.Add("region", host.Region)
	values.Add("network_id", network.ID)
	values.Add("tags", "inlets")

	body, err := apiCall(key, http.MethodPost, apiURL, strings.NewReader(values.Encode()))
	if err != nil {
		return instance, err
	}

	unmarshalErr := json.Unmarshal(body, &instance)
	if unmarshalErr != nil {
		return instance, unmarshalErr
	}

	fmt.Printf("Instance ID: %s\n", instance.ID)
	return instance, nil
}

func apiCall(key, method, url string, requestBody io.Reader) ([]byte, error) {

	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}
	addAuth(req, key)

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	var body []byte
	if res.Body != nil {
		defer res.Body.Close()
		body, err = ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP code: %d\n%q", res.StatusCode, string(body))
	}

	return body, nil
}

type apiResponse struct {
	Items []createdInstance `json:"items"`
}

type createdInstance struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	PublicIP  string    `json:"public_ip"`
	Status    string    `json:"status"`
}

func addAuth(r *http.Request, APIKey string) {
	r.Header.Add("Authorization", fmt.Sprintf("bearer %s", APIKey))
	r.Header.Add("User-Agent", "inlets")
}
