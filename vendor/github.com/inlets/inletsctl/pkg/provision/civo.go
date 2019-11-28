// Copyright (c) Inlets Author(s) 2019. All rights reserved.
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package provision

import (
	"encoding/json"
	"fmt"
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

// NewCivoProvisioner with an accessKey
func NewCivoProvisioner(accessKey string) (*CivoProvisioner, error) {

	return &CivoProvisioner{
		APIKey: accessKey,
	}, nil
}

func (p *CivoProvisioner) Status(id string) (*ProvisionedHost, error) {
	host := &ProvisionedHost{}

	apiURL := fmt.Sprint("https://api.civo.com/v2/instances/", id)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return host, err
	}
	addAuth(req, p.APIKey)

	req.Header.Add("Accept", "application/json")
	instance := CreatedInstance{}

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

func (p *CivoProvisioner) Delete(id string) error {

	apiURL := fmt.Sprint("https://api.civo.com/v2/instances/", id)

	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}
	addAuth(req, p.APIKey)

	req.Header.Add("Accept", "application/json")
	instance := CreatedInstance{}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	var body []byte
	if res.Body != nil {
		defer res.Body.Close()
		body, _ = ioutil.ReadAll(res.Body)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP code: %d\n%q", res.StatusCode, string(body))
	}

	unmarshalErr := json.Unmarshal(body, &instance)
	if unmarshalErr != nil {
		return unmarshalErr
	}
	return nil
}

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

func provisionCivoInstance(host BasicHost, key string) (CreatedInstance, error) {
	instance := CreatedInstance{}

	apiURL := "https://api.civo.com/v2/instances"

	values := url.Values{}
	values.Add("hostname", host.Name)
	values.Add("size", host.Plan)
	values.Add("public_ip", "true")
	values.Add("template_id", host.OS)
	values.Add("initial_user", "civo")
	values.Add("script", host.UserData)
	values.Add("tags", "inlets")

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(values.Encode()))
	if err != nil {
		return instance, err
	}
	addAuth(req, key)

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return instance, err
	}

	var body []byte
	if res.Body != nil {
		defer res.Body.Close()
		body, _ = ioutil.ReadAll(res.Body)
	}

	if res.StatusCode != http.StatusOK {
		return instance, fmt.Errorf("unexpected HTTP code: %d\n%q", res.StatusCode, string(body))
	}

	unmarshalErr := json.Unmarshal(body, &instance)
	if unmarshalErr != nil {
		return instance, unmarshalErr
	}

	fmt.Printf("Instance ID: %s\n", instance.ID)
	return instance, nil
}

type CreatedInstance struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	PublicIP  string    `json:"public_ip"`
	Status    string    `json:"status"`
}

func addAuth(r *http.Request, APIKey string) {
	r.Header.Add("Authorization", fmt.Sprintf("bearer %s", APIKey))
	r.Header.Add("User-Agent", "inlets")
}
