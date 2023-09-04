package provision

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws/credentials"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/uuid"
)

// EC2Provisioner contains the EC2 client
type EC2Provisioner struct {
	ec2Provisioner *ec2.EC2
}

// NewEC2Provisioner creates an EC2Provisioner and initialises an EC2 client
func NewEC2Provisioner(region, accessKey, secretKey, sessionToken string) (*EC2Provisioner, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, sessionToken),
	})
	svc := ec2.New(sess)
	return &EC2Provisioner{ec2Provisioner: svc}, err
}

// Provision deploys an exit node into AWS EC2
func (p *EC2Provisioner) Provision(host BasicHost) (*ProvisionedHost, error) {
	image, err := p.lookupAMI(host.OS)
	if err != nil {
		return nil, err
	}
	controlPort := 8123

	openHighPortsV := host.Additional["pro"]

	openHighPorts, _ := strconv.ParseBool(openHighPortsV)

	ports := host.Additional["ports"]

	keyName := host.Additional["key-name"]

	extraPorts, err := parsePorts(ports)
	if err != nil {
		return nil, err
	}

	var vpcID = host.Additional["vpc-id"]
	var subnetID = host.Additional["subnet-id"]

	groupID, name, err := p.createEC2SecurityGroup(vpcID, controlPort, openHighPorts, extraPorts)
	if err != nil {
		return nil, err
	}

	var networkSpec = ec2.InstanceNetworkInterfaceSpecification{
		DeviceIndex:              aws.Int64(int64(0)),
		AssociatePublicIpAddress: aws.Bool(true),
		DeleteOnTermination:      aws.Bool(true),
		Groups:                   []*string{groupID},
	}

	if len(subnetID) > 0 {
		networkSpec.SubnetId = aws.String(subnetID)
	}
	runInput := &ec2.RunInstancesInput{
		ImageId:      image,
		InstanceType: aws.String(host.Plan),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
		UserData:     &host.UserData,
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
			&networkSpec,
		},
	}

	if len(keyName) > 0 {
		runInput.KeyName = aws.String(keyName)
	}

	runResult, err := p.ec2Provisioner.RunInstances(runInput)
	if err != nil {
		// clean up SG if there was an issue provisioning the EC2 instance
		input := ec2.DeleteSecurityGroupInput{
			GroupId: groupID,
		}
		_, sgErr := p.ec2Provisioner.DeleteSecurityGroup(&input)
		if sgErr != nil {
			return nil, fmt.Errorf("error provisioning ec2 instance: %v; error deleting SG: %v", err, sgErr)
		}
		return nil, err
	}

	if len(runResult.Instances) == 0 {
		return nil, fmt.Errorf("could not create host: %s", runResult.String())
	}

	// AE: not sure why this error isn't handled?
	_, err = p.ec2Provisioner.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{runResult.Instances[0].InstanceId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(*name),
			},
			{
				Key:   aws.String("inlets"),
				Value: aws.String("exit-node"),
			},
		},
	})

	return &ProvisionedHost{
		ID:     *runResult.Instances[0].InstanceId,
		Status: "creating",
	}, nil
}

// Status returns the ID, Status and IP of the exit node
func (p *EC2Provisioner) Status(id string) (*ProvisionedHost, error) {
	var status string
	s, err := p.ec2Provisioner.DescribeInstanceStatus(&ec2.DescribeInstanceStatusInput{
		InstanceIds: []*string{aws.String(id)},
	})
	if err != nil {
		return nil, err
	}
	if len(s.InstanceStatuses) > 0 {
		if *s.InstanceStatuses[0].InstanceStatus.Status == "ok" {
			status = ActiveStatus
		} else {
			status = "initialising"
		}
	} else {
		status = "creating"
	}

	d, err := p.ec2Provisioner.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(id)},
	})
	if err != nil {
		return nil, err
	}
	if len(d.Reservations) == 0 {
		return nil, fmt.Errorf("cannot describe host: %s", id)
	}

	return &ProvisionedHost{
		ID:     id,
		Status: status,
		IP:     aws.StringValue(d.Reservations[0].Instances[0].PublicIpAddress),
	}, nil
}

// Delete removes the exit node
func (p *EC2Provisioner) Delete(request HostDeleteRequest) error {
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

	i, err := p.ec2Provisioner.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(id)},
	})
	if err != nil {
		return err
	}
	groups := i.Reservations[0].Instances[0].SecurityGroups

	_, err = p.ec2Provisioner.TerminateInstances(&ec2.TerminateInstancesInput{
		InstanceIds: []*string{aws.String(id)},
	})
	if err != nil {
		return err
	}

	// Instance has to be terminated before we can remove the security group
	err = p.ec2Provisioner.WaitUntilInstanceTerminated(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(id)},
	})
	if err != nil {
		return err
	}

	for _, group := range groups {
		_, err := p.ec2Provisioner.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
			GroupId: group.GroupId,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// List returns a list of exit nodes
func (p *EC2Provisioner) List(filter ListFilter) ([]*ProvisionedHost, error) {
	var inlets []*ProvisionedHost
	var nextToken *string
	filterValues := strings.Split(filter.Filter, ",")
	for {
		instances, err := p.ec2Provisioner.DescribeInstances(&ec2.DescribeInstancesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String(filterValues[0]),
					Values: []*string{aws.String(filterValues[1])},
				},
			},
			NextToken: nextToken,
		})
		if err != nil {
			return nil, err
		}
		for _, r := range instances.Reservations {
			for _, i := range r.Instances {
				if *i.State.Name != ec2.InstanceStateNameTerminated {
					host := &ProvisionedHost{
						ID: *i.InstanceId,
					}
					if i.PublicIpAddress != nil {
						host.IP = *i.PublicIpAddress
					}
					inlets = append(inlets, host)
				}
			}
		}
		nextToken = instances.NextToken
		if nextToken == nil {
			break
		}
	}
	return inlets, nil
}

func (p *EC2Provisioner) lookupID(request HostDeleteRequest) (string, error) {
	inlets, err := p.List(ListFilter{
		Filter:    "tag:inlets,exit-node",
		ProjectID: request.ProjectID,
		Zone:      request.Zone,
	})
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

// createEC2SecurityGroup creates a security group for the exit-node
func (p *EC2Provisioner) createEC2SecurityGroup(vpcID string, controlPort int, openHighPorts bool, extraPorts []int) (*string, *string, error) {
	ports := []int{controlPort}

	highPortRange := []int{1024, 65535}

	if len(extraPorts) > 0 {
		// disable high port range if extra ports are specified
		highPortRange = []int{}

		ports = append(ports, extraPorts...)
	}

	groupName := "inlets-" + uuid.New().String()
	var input = &ec2.CreateSecurityGroupInput{
		Description: aws.String("inlets security group"),
		GroupName:   aws.String(groupName),
	}

	if len(vpcID) > 0 {
		input.VpcId = aws.String(vpcID)
	}

	group, err := p.ec2Provisioner.CreateSecurityGroup(input)
	if err != nil {
		return nil, nil, err
	}

	for _, port := range ports {
		if err = p.createEC2SecurityGroupRule(*group.GroupId, port, port); err != nil {
			return group.GroupId, &groupName,
				fmt.Errorf("failed to create security group on %s with port %d: %w", *group.GroupId, port, err)
		}
	}

	if openHighPorts && len(highPortRange) == 2 {
		err = p.createEC2SecurityGroupRule(*group.GroupId, highPortRange[0], highPortRange[1])
		if err != nil {
			return group.GroupId, &groupName, err
		}
	}

	return group.GroupId, &groupName, nil
}

func parsePorts(extraPorts string) ([]int, error) {
	var ports []int

	parts := strings.Split(extraPorts, ",")
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); len(trimmed) > 0 {
			port, err := strconv.Atoi(trimmed)
			if err != nil {
				return nil, err
			}
			ports = append(ports, port)
		}
	}

	return ports, nil
}

func (p *EC2Provisioner) createEC2SecurityGroupRule(groupID string, fromPort, toPort int) error {
	_, err := p.ec2Provisioner.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		CidrIp:     aws.String("0.0.0.0/0"),
		FromPort:   aws.Int64(int64(fromPort)),
		IpProtocol: aws.String("tcp"),
		ToPort:     aws.Int64(int64(toPort)),
		GroupId:    aws.String(groupID),
	})
	if err != nil {
		return err
	}
	return nil
}

// lookupAMI gets the AMI ID that the exit node will use
func (p *EC2Provisioner) lookupAMI(name string) (*string, error) {
	images, err := p.ec2Provisioner.DescribeImages(&ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("name"),
				Values: []*string{
					aws.String(name),
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	if len(images.Images) == 0 {
		return nil, fmt.Errorf("image not found")
	}
	return images.Images[0].ImageId, nil
}
