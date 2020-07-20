package main

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

func validateFlags(c InfraConfig) error {
	if c.Provider == "packet" {
		if len(c.ProjectID) == 0 {
			return fmt.Errorf("project-id required for provider: %s", c.Provider)
		}
	}
	if c.Provider == "gce" {
		if len(c.ProjectID) == 0 {
			return fmt.Errorf("project-id required for provider: %s", c.Provider)
		}
		if len(c.Zone) == 0 {
			return fmt.Errorf("zone required for provider: %s", c.Provider)
		}
	}
	if c.Provider == "azure" {
		if len(c.SubscriptionID) == 0 {
			return fmt.Errorf("subscription-id required for provider: %s", c.Provider)
		}
	}
	if len(c.MaxClientMemory) > 0 {
		if _, err := resource.ParseQuantity(c.MaxClientMemory); err != nil {
			return fmt.Errorf("invalid memory value: %s", err.Error())
		}
	}

	return nil
}
