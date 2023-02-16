package provision

// Provisioner is an interface used for deploying exit nodes into cloud providers
type Provisioner interface {
	Provision(BasicHost) (*ProvisionedHost, error)
	Status(id string) (*ProvisionedHost, error)
	Delete(HostDeleteRequest) error
}

// ActiveStatus is the status returned by an active exit node
const ActiveStatus = "active"

// ProvisionedHost contains the IP, ID and Status of an exit node
type ProvisionedHost struct {
	IP     string
	ID     string
	Status string
}

// BasicHost contains the data required to deploy a exit node
type BasicHost struct {
	Region     string
	Plan       string
	OS         string
	Name       string
	UserData   string
	Additional map[string]string
}

// HostDeleteRequest contains the data required to delete an exit node by either IP or ID
type HostDeleteRequest struct {
	ID        string
	IP        string
	ProjectID string
	Zone      string
}

// ListFilter is used to filter results to return only exit nodes
type ListFilter struct {
	Filter    string
	ProjectID string
	Zone      string
}
