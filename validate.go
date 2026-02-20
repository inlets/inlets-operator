package main

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

func validateFlags(c InfraConfig) error {
	if len(c.Provider) == 0 {
		return fmt.Errorf("provider flag is required")
	}

	if c.Provider == "equinix-metal" {
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
		if len(c.Region) == 0 {
			return fmt.Errorf("region required for provider: %s", c.Provider)
		}
	}
	if c.Provider == "azure" {
		if len(c.SubscriptionID) == 0 {
			return fmt.Errorf("subscription-id required for provider: %s", c.Provider)
		}
	}
	if c.Provider == "hetzner" {
		if len(c.Region) == 0 {
			return fmt.Errorf("region required for provider: %s", c.Provider)
		}
	}

	if len(c.MaxClientMemory) > 0 {
		if _, err := resource.ParseQuantity(c.MaxClientMemory); err != nil {
			return fmt.Errorf("invalid memory value: %s", err.Error())
		}
	}

	if len(c.AccessKey) == 0 && len(c.AccessKeyFile) == 0 {
		return fmt.Errorf("access-key or access-key-file must be given")
	}

	return nil
}
