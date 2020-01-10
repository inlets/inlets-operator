package provision

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/uuid"
)

// EC2Provisioner contains the EC2 client
type EC2Provisioner struct {
	ec2Provisioner *ec2.EC2
}

// NewEC2Provioner creates an EC2Provisioner and initialises an EC2 client
func NewEC2Provisioner(region, accessKey, secretKey string) (*EC2Provisioner, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
	})
	svc := ec2.New(sess)
	return &EC2Provisioner{ec2Provisioner: svc}, err
}

// Provision deploys an exit node into AWS
func (p *EC2Provisioner) Provision(host BasicHost) (*ProvisionedHost, error) {
	image, err := p.lookupAMI(host.OS)
	if err != nil {
		return nil, err
	}

	groupID, name, err := p.securityGroup(host.Additional["inlets-port"])
	if err != nil {
		return nil, err
	}

	runResult, err := p.ec2Provisioner.RunInstances(&ec2.RunInstancesInput{
		ImageId:      image,
		InstanceType: aws.String(host.Plan),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
		UserData:     &host.UserData,
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
			{
				DeviceIndex:              aws.Int64(int64(0)),
				AssociatePublicIpAddress: aws.Bool(true),
				DeleteOnTermination:      aws.Bool(true),
				Groups:                   []*string{groupID},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	if len(runResult.Instances) == 0 {
		return nil, fmt.Errorf("could not create host: %s", runResult.String())
	}

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
func (p *EC2Provisioner) Delete(id string) error {
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

// securityGroup creates a security group and ingress rule for the inlets port
func (p *EC2Provisioner) securityGroup(port string) (*string, *string, error) {
	targetPort, err := strconv.Atoi(port)
	if err != nil {
		return nil, nil, err
	}
	groupName := "inlets-" + uuid.New().String()
	group, err := p.ec2Provisioner.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
		Description: aws.String("inlets security group"),
		GroupName:   aws.String(groupName),
	})
	if err != nil {
		return nil, nil, err
	}

	_, err = p.ec2Provisioner.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		CidrIp:     aws.String("0.0.0.0/0"),
		FromPort:   aws.Int64(int64(targetPort)),
		IpProtocol: aws.String("TCP"),
		ToPort:     aws.Int64(int64(targetPort)),
		GroupId:    group.GroupId,
	})
	if err != nil {
		return nil, nil, err
	}

	return group.GroupId, &groupName, nil
}

// lookupAMI gets the AMI ID that the exit node will use

func (p *EC2Provisioner) lookupAMI(name string) (*string, error) {
	images, err := p.ec2Provisioner.DescribeImages(&ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
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
