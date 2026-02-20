package main

import (
	"fmt"
	"io/ioutil"
	"strings"
)

// InfraConfig is the configuration for
// creating Infrastructure Resources
type InfraConfig struct {
	Provider        string
	Region          string
	Zone            string
	AccessKey       string
	SecretKey       string
	OrganizationID  string
	SubscriptionID  string
	VpcID           string
	SubnetID        string
	AccessKeyFile   string
	SecretKeyFile   string
	ProjectID       string
	AnnotatedOnly   bool
	MaxClientMemory string
	Plan            string
	TunnelConfig    TunnelConfig
}

type TunnelConfig struct {
	License       string
	LicenseFile   string
	ClientImage   string
	InletsRelease string
}

func (c TunnelConfig) GetLicenseKey() (string, error) {
	val := ""
	if len(c.License) > 0 {
		val = c.License
	} else {
		data, err := ioutil.ReadFile(c.LicenseFile)
		if err != nil {
			return "", fmt.Errorf("error with GetLicenseKey: %s", err.Error())
		}
		val = string(data)
	}

	if len(val) == 0 {
		return "", fmt.Errorf("--license or --license-key is required for inlets PRO")
	}

	if dots := strings.Count(val, "."); dots >= 2 {
		return strings.TrimSpace(val), nil
	}

	if dashes := strings.Count(val, "-"); dashes == 3 {
		return strings.TrimSpace(val), nil
	}

	if dashes := strings.Count(val, "-"); dashes == 4 {
		return strings.TrimSpace(val), nil
	}

	return "", fmt.Errorf("inlets license may be invalid")
}
