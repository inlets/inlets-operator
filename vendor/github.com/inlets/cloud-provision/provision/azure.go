package provision

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"unicode/utf16"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/dimchansky/utfbom"
	"github.com/google/uuid"
	"github.com/sethvargo/go-password/password"
)

const AzureStatusSucceeded = "Succeeded"

type AzureProvisioner struct {
	subscriptionId       string
	resourceGroupName    string
	deploymentName       string
	azidentityCredential *azidentity.EnvironmentCredential
	ctx                  context.Context
}

var fileToEnvMap = map[string]string{
	"subscriptionId":             "AZURE_SUBSCRIPTION_ID",
	"tenantId":                   "AZURE_TENANT_ID",
	"auxiliaryTenantIds":         "AZURE_AUXILIARY_TENANT_IDS",
	"clientId":                   "AZURE_CLIENT_ID",
	"clientSecret":               "AZURE_CLIENT_SECRET",
	"certificatePath":            "AZURE_CERTIFICATE_PATH",
	"certificatePassword":        "AZURE_CERTIFICATE_PASSWORD",
	"username":                   "AZURE_USERNAME",
	"password":                   "AZURE_PASSWORD",
	"environmentName":            "AZURE_ENVIRONMENT",
	"resource":                   "AZURE_AD_RESOURCE",
	"activeDirectoryEndpointUrl": "ActiveDirectoryEndpoint",
	"resourceManagerEndpointUrl": "ResourceManagerEndpoint",
	"graphResourceId":            "GraphResourceID",
	"sqlManagementEndpointUrl":   "SQLManagementEndpoint",
	"galleryEndpointUrl":         "GalleryEndpoint",
	"managementEndpointUrl":      "ManagementEndpoint",
}

// buildAzureHostID creates an ID for Azure based upon the group name,
// and deployment name
func buildAzureHostID(groupName, deploymentName string) (id string) {
	return fmt.Sprintf("%s|%s", groupName, deploymentName)
}

// get some required fields from the custom Azure host ID
func getAzureFieldsFromID(id string) (groupName, deploymentName string, err error) {
	fields := strings.Split(id, "|")
	err = nil
	if len(fields) == 2 {
		groupName = fields[0]
		deploymentName = fields[1]
	} else {
		err = fmt.Errorf("could not get fields from custom ID: fields: %v", fields)
		return "", "", err
	}
	return groupName, deploymentName, nil
}

// In case azure auth file is encoded as UTF-16 instead of UTF-8
func decodeAzureAuthContents(b []byte) ([]byte, error) {
	reader, enc := utfbom.Skip(bytes.NewReader(b))

	switch enc {
	case utfbom.UTF16LittleEndian:
		u16 := make([]uint16, (len(b)/2)-1)
		err := binary.Read(reader, binary.LittleEndian, &u16)
		if err != nil {
			return nil, err
		}
		return []byte(string(utf16.Decode(u16))), nil
	case utfbom.UTF16BigEndian:
		u16 := make([]uint16, (len(b)/2)-1)
		err := binary.Read(reader, binary.BigEndian, &u16)
		if err != nil {
			return nil, err
		}
		return []byte(string(utf16.Decode(u16))), nil
	}
	return ioutil.ReadAll(reader)
}

func NewAzureProvisioner(subscriptionId, authFileContents string) (*AzureProvisioner, error) {
	decodedAuthContents, err := decodeAzureAuthContents([]byte(authFileContents))
	if err != nil {
		log.Printf("Failed to decode auth contents: '%s', error: '%s'", authFileContents, err.Error())
		return nil, err
	}
	authMap := map[string]string{}
	err = json.Unmarshal(decodedAuthContents, &authMap)
	if err != nil {
		log.Printf("Failed to parse auth contents: '%s', error: '%s'", authFileContents, err.Error())
		return nil, err
	}
	for fileKey, envKey := range fileToEnvMap {
		err := os.Setenv(envKey, authMap[fileKey])
		if err != nil {
			log.Printf("Failed to set env: '%s', error: '%s'", fileKey, err.Error())
		}
	}
	credential, err := azidentity.NewEnvironmentCredential(nil)
	ctx := context.Background()
	return &AzureProvisioner{
		subscriptionId:       subscriptionId,
		azidentityCredential: credential,
		ctx:                  ctx,
	}, err
}

// Provision provisions a new Azure instance as an exit node
func (p *AzureProvisioner) Provision(host BasicHost) (*ProvisionedHost, error) {

	log.Printf("Provisioning host with Azure\n")

	p.resourceGroupName = "inlets-" + host.Name
	p.deploymentName = "deployment-" + uuid.New().String()

	log.Printf("Creating resource group %s", p.resourceGroupName)
	group, err := createGroup(p, host)
	if err != nil {
		return nil, err
	}
	log.Printf("Resource group created %s", *group.Name)

	log.Printf("Creating deployment %s", p.deploymentName)
	err = createDeployment(p, host)
	if err != nil {
		return nil, err
	}
	return &ProvisionedHost{
		IP:     "",
		ID:     buildAzureHostID(p.resourceGroupName, p.deploymentName),
		Status: ActiveStatus,
	}, nil
}

// Status checks the status of the provisioning Azure exit node
func (p *AzureProvisioner) Status(id string) (*ProvisionedHost, error) {
	deploymentsClient, err := armresources.NewDeploymentsClient(p.subscriptionId, p.azidentityCredential, nil)
	if err != nil {
		return nil, err
	}

	resourceGroupName, deploymentName, err := getAzureFieldsFromID(id)
	if err != nil {
		return nil, err
	}

	deployment, err := deploymentsClient.Get(p.ctx, resourceGroupName, deploymentName, nil)
	if err != nil {
		return nil, err
	}
	var deploymentStatus string
	if *deployment.Properties.ProvisioningState == AzureStatusSucceeded {
		deploymentStatus = ActiveStatus
	} else {
		deploymentStatus = string(*deployment.Properties.ProvisioningState)
	}
	IP := ""
	if deploymentStatus == ActiveStatus {
		IP = deployment.Properties.Outputs.(map[string]interface{})["publicIP"].(map[string]interface{})["value"].(string)
	}
	return &ProvisionedHost{
		IP:     IP,
		ID:     id,
		Status: deploymentStatus,
	}, nil
}

// Delete deletes the Azure exit node
func (p *AzureProvisioner) Delete(request HostDeleteRequest) error {
	groupsClient, err := armresources.NewResourceGroupsClient(p.subscriptionId, p.azidentityCredential, nil)
	if err != nil {
		return err
	}
	resourceGroupName, _, err := getAzureFieldsFromID(request.ID)
	if err != nil {
		return err
	}
	_, err = groupsClient.BeginDelete(p.ctx, resourceGroupName, nil)
	return err
}

func createGroup(p *AzureProvisioner, host BasicHost) (*armresources.ResourceGroup, error) {
	groupsClient, err := armresources.NewResourceGroupsClient(p.subscriptionId, p.azidentityCredential, nil)
	if err != nil {
		return nil, err
	}
	resourceGroupResp, err := groupsClient.CreateOrUpdate(
		p.ctx,
		p.resourceGroupName,
		armresources.ResourceGroup{Location: to.Ptr(host.Region)}, nil)

	if err != nil {
		return nil, err
	}
	return &resourceGroupResp.ResourceGroup, nil
}

func getSecurityRuleList(host BasicHost) []interface{} {
	var rules []interface{}
	if host.Additional["pro"] == "true" {
		rules = []interface{}{
			getSecurityRule("AllPorts", 280, "TCP", "*"),
		}
	} else {
		rules = []interface{}{
			getSecurityRule("HTTPS", 320, "TCP", "443"),
			getSecurityRule("HTTP", 340, "TCP", "80"),
			getSecurityRule("HTTP8080", 360, "TCP", "8080"),
		}
	}
	return rules
}

func getSecurityRule(name string, priority int, protocol, destPortRange string) map[string]interface{} {
	return map[string]interface{}{
		"name": name,
		"properties": map[string]interface{}{
			"priority":                 priority,
			"protocol":                 protocol,
			"access":                   "Allow",
			"direction":                "Inbound",
			"sourceAddressPrefix":      "*",
			"sourcePortRange":          "*",
			"destinationAddressPrefix": "*",
			"destinationPortRange":     destPortRange,
		},
	}
}

func azureParameterType(typeName string) map[string]interface{} {
	return map[string]interface{}{
		"type": typeName,
	}
}

func azureParameterValue(typeValue string) map[string]interface{} {
	return map[string]interface{}{
		"value": typeValue,
	}
}

func getTemplateParameterDefinition() map[string]interface{} {
	return map[string]interface{}{
		"location":                  azureParameterType("string"),
		"networkInterfaceName":      azureParameterType("string"),
		"networkSecurityGroupName":  azureParameterType("string"),
		"networkSecurityGroupRules": azureParameterType("array"),
		"subnetName":                azureParameterType("string"),
		"virtualNetworkName":        azureParameterType("string"),
		"addressPrefixes":           azureParameterType("array"),
		"subnets":                   azureParameterType("array"),
		"publicIpAddressName":       azureParameterType("string"),
		"virtualMachineName":        azureParameterType("string"),
		"virtualMachineRG":          azureParameterType("string"),
		"osDiskType":                azureParameterType("string"),
		"virtualMachineSize":        azureParameterType("string"),
		"adminUsername":             azureParameterType("string"),
		"adminPassword":             azureParameterType("secureString"),
		"customData":                azureParameterType("string"),
	}
}

func getTemplateResourceVirtualMachine(host BasicHost) map[string]interface{} {
	return map[string]interface{}{
		"name":       "[parameters('virtualMachineName')]",
		"type":       "Microsoft.Compute/virtualMachines",
		"apiVersion": "2019-07-01",
		"location":   "[parameters('location')]",
		"dependsOn": []interface{}{
			"[concat('Microsoft.Network/networkInterfaces/', parameters('networkInterfaceName'))]",
		},
		"properties": map[string]interface{}{
			"hardwareProfile": map[string]interface{}{
				"vmSize": "[parameters('virtualMachineSize')]",
			},
			"storageProfile": map[string]interface{}{
				"osDisk": map[string]interface{}{
					"createOption": "fromImage",
					"managedDisk": map[string]interface{}{
						"storageAccountType": "[parameters('osDiskType')]",
					},
				},
				"imageReference": map[string]interface{}{
					"publisher": host.Additional["imagePublisher"],
					"offer":     host.Additional["imageOffer"],
					"sku":       host.Additional["imageSku"],
					"version":   host.Additional["imageVersion"],
				},
			},
			"networkProfile": map[string]interface{}{
				"networkInterfaces": []interface{}{
					map[string]interface{}{
						"id": "[resourceId('Microsoft.Network/networkInterfaces', parameters('networkInterfaceName'))]",
					},
				},
			},
			"osProfile": map[string]interface{}{
				"computerName":  "[parameters('virtualMachineName')]",
				"adminUsername": "[parameters('adminUsername')]",
				"adminPassword": "[parameters('adminPassword')]",
				"customData":    "[base64(parameters('customData'))]",
			},
		},
	}
}

func getTemplateResourceNetworkInterface() map[string]interface{} {
	return map[string]interface{}{
		"name":       "[parameters('networkInterfaceName')]",
		"type":       "Microsoft.Network/networkInterfaces",
		"apiVersion": "2019-07-01",
		"location":   "[parameters('location')]",
		"dependsOn": []interface{}{
			"[concat('Microsoft.Network/networkSecurityGroups/', parameters('networkSecurityGroupName'))]",
			"[concat('Microsoft.Network/virtualNetworks/', parameters('virtualNetworkName'))]",
			"[concat('Microsoft.Network/publicIpAddresses/', parameters('publicIpAddressName'))]",
		},
		"properties": map[string]interface{}{
			"ipConfigurations": []interface{}{
				map[string]interface{}{
					"name": "ipconfig1",
					"properties": map[string]interface{}{
						"subnet": map[string]interface{}{
							"id": "[variables('subnetRef')]",
						},
						"privateIPAllocationMethod": "Dynamic",
						"publicIpAddress": map[string]interface{}{
							"id": "[resourceId(resourceGroup().name, 'Microsoft.Network/publicIpAddresses', parameters('publicIpAddressName'))]",
						},
					},
				},
			},
			"networkSecurityGroup": map[string]interface{}{
				"id": "[variables('nsgId')]",
			},
		},
	}
}

func getTemplate(host BasicHost) map[string]interface{} {
	return map[string]interface{}{
		"$schema":        "http://schema.management.azure.com/schemas/2015-01-01/deploymentTemplate.json#",
		"contentVersion": "1.0.0.0",
		"parameters":     getTemplateParameterDefinition(),
		"variables": map[string]interface{}{
			"nsgId":     "[resourceId(resourceGroup().name, 'Microsoft.Network/networkSecurityGroups', parameters('networkSecurityGroupName'))]",
			"vnetId":    "[resourceId(resourceGroup().name,'Microsoft.Network/virtualNetworks', parameters('virtualNetworkName'))]",
			"subnetRef": "[concat(variables('vnetId'), '/subnets/', parameters('subnetName'))]",
		},
		"resources": []interface{}{
			getTemplateResourceNetworkInterface(),
			map[string]interface{}{
				"name":       "[parameters('networkSecurityGroupName')]",
				"type":       "Microsoft.Network/networkSecurityGroups",
				"apiVersion": "2019-02-01",
				"location":   host.Region,
				"properties": map[string]interface{}{
					"securityRules": "[parameters('networkSecurityGroupRules')]",
				},
			},
			map[string]interface{}{
				"name":       "[parameters('virtualNetworkName')]",
				"type":       "Microsoft.Network/virtualNetworks",
				"apiVersion": "2019-04-01",
				"location":   host.Region,
				"properties": map[string]interface{}{
					"addressSpace": map[string]interface{}{
						"addressPrefixes": "[parameters('addressPrefixes')]",
					},
					"subnets": "[parameters('subnets')]",
				},
			},
			map[string]interface{}{
				"name":       "[parameters('publicIpAddressName')]",
				"type":       "Microsoft.Network/publicIpAddresses",
				"apiVersion": "2019-02-01",
				"location":   host.Region,
				"properties": map[string]interface{}{
					"publicIpAllocationMethod": armnetwork.IPAllocationMethodStatic,
				},
				"sku": map[string]interface{}{
					"name": armnetwork.PublicIPAddressSKUNameBasic,
				},
			},
			getTemplateResourceVirtualMachine(host),
		},
		"outputs": map[string]interface{}{
			"adminUsername": map[string]interface{}{
				"type":  "string",
				"value": "[parameters('adminUsername')]",
			},
			"publicIP": map[string]interface{}{
				"type":  "string",
				"value": "[reference(resourceId('Microsoft.Network/publicIPAddresses', parameters('publicIpAddressName')), '2019-02-01', 'Full').properties.ipAddress]",
				// See also https://docs.microsoft.com/en-us/azure/azure-resource-manager/templates/template-functions-resource#reference
				// and https://docs.microsoft.com/en-us/azure/templates/microsoft.network/2019-02-01/publicipaddresses
			},
		},
	}
}

func getParameters(p *AzureProvisioner, host BasicHost) (parameters map[string]interface{}) {
	return map[string]interface{}{
		"location":                 azureParameterValue(host.Region),
		"networkInterfaceName":     azureParameterValue("inlets-vm-nic"),
		"networkSecurityGroupName": azureParameterValue("inlets-vm-nsg"),
		"networkSecurityGroupRules": map[string]interface{}{
			"value": getSecurityRuleList(host),
		},
		"subnetName":         azureParameterValue("default"),
		"virtualNetworkName": azureParameterValue("inlets-vnet"),
		"addressPrefixes": map[string]interface{}{
			"value": []interface{}{
				"10.0.0.0/24",
			},
		},
		"subnets": map[string]interface{}{
			"value": []interface{}{
				map[string]interface{}{
					"name": "default",
					"properties": map[string]interface{}{
						"addressPrefix": "10.0.0.0/24",
					},
				},
			},
		},
		"publicIpAddressName": azureParameterValue("inlets-ip"),
		"virtualMachineName":  azureParameterValue(host.Name),
		"virtualMachineRG":    azureParameterValue(p.resourceGroupName),
		"osDiskType": map[string]interface{}{
			"value": armcompute.StorageAccountTypesStandardLRS,
		},
		"virtualMachineSize": azureParameterValue(host.Plan),
		"adminUsername":      azureParameterValue("inletsuser"),
		"adminPassword":      azureParameterValue(host.Additional["adminPassword"]),
		"customData":         azureParameterValue(host.UserData),
	}
}

func createDeployment(p *AzureProvisioner, host BasicHost) (err error) {
	adminPassword, err := password.Generate(16, 4, 0, false, true)
	if err != nil {
		return
	}
	host.Additional["adminPassword"] = adminPassword
	template := getTemplate(host)
	params := getParameters(p, host)
	deploymentsClient, err := armresources.NewDeploymentsClient(p.subscriptionId, p.azidentityCredential, nil)
	if err != nil {
		return
	}

	_, err = deploymentsClient.BeginCreateOrUpdate(
		p.ctx,
		p.resourceGroupName,
		p.deploymentName,
		armresources.Deployment{
			Properties: &armresources.DeploymentProperties{
				Template:   template,
				Parameters: params,
				Mode:       to.Ptr(armresources.DeploymentModeComplete),
			},
		},
		nil,
	)
	return
}
