package linodego

import (
	"context"
	"encoding/json"
	"time"

	"github.com/linode/linodego/internal/parseabletime"
)

// AccountMaintenance represents a Maintenance object for any entity a user has permissions to view
type AccountMaintenance struct {
	Entity *Entity    `json:"entity"`
	Reason string     `json:"reason"`
	Status string     `json:"status"`
	Type   string     `json:"type"`
	When   *time.Time `json:"when"`
}

// The entity being affected by maintenance
type Entity struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"`
	URL   string `json:"url"`
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (accountMaintenance *AccountMaintenance) UnmarshalJSON(b []byte) error {
	type Mask AccountMaintenance

	p := struct {
		*Mask
		When *parseabletime.ParseableTime `json:"when"`
	}{
		Mask: (*Mask)(accountMaintenance),
	}

	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}

	accountMaintenance.When = (*time.Time)(p.When)

	return nil
}

// ListMaintenances lists Account Maintenance objects for any entity a user has permissions to view
func (c *Client) ListMaintenances(ctx context.Context, opts *ListOptions) ([]AccountMaintenance, error) {
	return getPaginatedResults[AccountMaintenance](ctx, c, "account/maintenance", opts)
}
