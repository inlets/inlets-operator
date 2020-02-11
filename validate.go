package main

import "fmt"

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
	return nil
}
