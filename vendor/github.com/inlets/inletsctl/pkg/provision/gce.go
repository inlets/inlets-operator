package provision

import (
	"context"
	"fmt"
	"log"
	"strings"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// GCEProvisioner holds reference to the compute service to provision compute resources
type GCEProvisioner struct {
	gceProvisioner *compute.Service
}

// NewGCEProvisioner returns a new GCEProvisioner
func NewGCEProvisioner(accessKey string) (*GCEProvisioner, error) {
	gceService, err := compute.NewService(context.Background(), option.WithCredentialsJSON([]byte(accessKey)))
	return &GCEProvisioner{
		gceProvisioner: gceService,
	}, err
}

// Provision provisions a new GCE instance as an exit node
func (gce *GCEProvisioner) Provision(host BasicHost) (*ProvisionedHost, error) {
	// instance auto restart on failure
	autoRestart := true
	instance := &compute.Instance{
		Name:         host.Name,
		Description:  "Exit node created by inlets-operator",
		MachineType:  fmt.Sprintf("zones/%s/machineTypes/%s", host.Additional["zone"], host.Plan),
		CanIpForward: true,
		Zone:         fmt.Sprintf("projects/%s/zones/%s", host.Additional["projectid"], host.Additional["zone"]),
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				DeviceName: host.Name,
				Mode:       "READ_WRITE",
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					Description: "Boot Disk for the exit-node created by inlets-operator",
					DiskName:    host.Name,
					DiskSizeGb:  10,
					SourceImage: host.OS,
				},
			},
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				{
					Key:   "startup-script",
					Value: &host.UserData,
				},
			},
		},
		Tags: &compute.Tags{
			Items: []string{"http-server", "https-server", "inlets"},
		},
		Scheduling: &compute.Scheduling{
			AutomaticRestart:  &autoRestart,
			OnHostMaintenance: "MIGRATE",
			Preemptible:       false,
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
				Network: "global/networks/default",
			},
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email: "default",
				Scopes: []string{
					compute.ComputeScope,
				},
			},
		},
	}

	exists, _ := gce.checkInletsFirewallRuleExists(host.Additional["projectid"], host.Additional["firewall-name"], host.Additional["firewall-port"])

	if !exists {
		err := gce.createInletsFirewallRule(host.Additional["projectid"], host.Additional["firewall-name"], host.Additional["firewall-port"])
		log.Println("inlets firewallRule does not exist")
		if err != nil {
			return nil, fmt.Errorf("Could not create inlets firewall rule: %v", err)
		}
		log.Printf("Creating inlets firewallRule opening port: %s\n", host.Additional["firewall-port"])
	} else {
		log.Println("inlets firewallRule exists")
	}

	op, err := gce.gceProvisioner.Instances.Insert(host.Additional["projectid"], host.Additional["zone"], instance).Do()
	if err != nil {
		return nil, fmt.Errorf("could not provision GCE instance: %v", err)
	}

	status := ""

	if op.Status == "RUNNING" {
		status = ActiveStatus
	}
	return &ProvisionedHost{
		ID:     constructCustomGCEID(host.Name, host.Additional["zone"], host.Additional["projectid"]),
		Status: status,
	}, nil

}

// checkInletsFirewallRuleExists checks if the inlets firewall rule exists or not
func (gce *GCEProvisioner) checkInletsFirewallRuleExists(projectID string, firewallRuleName string, inletsPort string) (bool, error) {
	op, err := gce.gceProvisioner.Firewalls.Get(projectID, firewallRuleName).Do()
	if err != nil {
		return false, fmt.Errorf("Could not get inlets firewall rule: %v", err)
	}
	if op.Name == firewallRuleName {
		for _, firewallRule := range op.Allowed {
			for _, port := range firewallRule.Ports {
				if port == inletsPort {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// createInletsFirewallRule creates a firewall rule opening up the control port for inlets
func (gce *GCEProvisioner) createInletsFirewallRule(projectID string, firewallRuleName string, inletsPort string) error {
	firewallRule := &compute.Firewall{
		Name:        firewallRuleName,
		Description: "Firewall rule created by inlets-operator",
		Network:     fmt.Sprintf("projects/%s/global/networks/default", projectID),
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "tcp",
				Ports:      []string{inletsPort},
			},
		},
		SourceRanges: []string{"0.0.0.0/0"},
		Direction:    "INGRESS",
		TargetTags:   []string{"inlets"},
	}

	_, err := gce.gceProvisioner.Firewalls.Insert(projectID, firewallRule).Do()
	if err != nil {
		return fmt.Errorf("could not create firewall rule: %v", err)
	}
	return nil
}

// Delete deletes the GCE exit node
func (gce *GCEProvisioner) Delete(id string) error {
	instanceName, zone, projectID, err := getGCEFieldsFromID(id)
	if err != nil {
		return fmt.Errorf("Could not get custom GCE fields: %v", err)
	}
	_, err = gce.gceProvisioner.Instances.Delete(projectID, zone, instanceName).Do()
	if err != nil {
		return fmt.Errorf("Could not delete the GCE instance: %v", err)
	}
	return err
}

// Status checks the status of the provisioning GCE exit node
func (gce *GCEProvisioner) Status(id string) (*ProvisionedHost, error) {
	instanceName, zone, projectID, err := getGCEFieldsFromID(id)
	if err != nil {
		return nil, fmt.Errorf("Could not get custom GCE fields: %v", err)
	}

	op, err := gce.gceProvisioner.Instances.Get(projectID, zone, instanceName).Do()
	if err != nil {
		return nil, fmt.Errorf("Could not get instance: %v", err)
	}

	status := ""

	if op.Status == "RUNNING" {
		status = ActiveStatus
	}

	return &ProvisionedHost{
		IP:     op.NetworkInterfaces[0].AccessConfigs[0].NatIP,
		ID:     constructCustomGCEID(instanceName, zone, projectID),
		Status: status,
	}, nil
}

// construct custom GCE instance ID from fields
func constructCustomGCEID(instanceName, zone, projectID string) (id string) {
	return fmt.Sprintf("%s|%s|%s", instanceName, zone, projectID)
}

// get some required fields from the custom GCE instance ID
func getGCEFieldsFromID(id string) (instanceName, zone, projectID string, err error) {
	fields := strings.Split(id, "|")
	err = nil
	if len(fields) == 3 {
		instanceName = fields[0]
		zone = fields[1]
		projectID = fields[2]
	} else {
		err = fmt.Errorf("could not get fields from custom ID: fields: %v", fields)
		return "", "", "", err
	}
	return instanceName, zone, projectID, nil
}
