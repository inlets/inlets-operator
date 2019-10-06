package provision

type Provisioner interface {
	Provision(BasicHost) (*ProvisionedHost, error)
	Status(id string) (*ProvisionedHost, error)
	Delete(id string) error
}

type ProvisionedHost struct {
	IP     string
	ID     string
	Status string
}

type BasicHost struct {
	Region     string
	Plan       string
	OS         string
	Name       string
	UserData   string
	Additional map[string]string
}
