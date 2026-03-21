package models

import "time"

type Credential struct {
	LeaseID     string    `json:"lease_id"`
	Username    string    `json:"username"`
	RoleName    string    `json:"role_name"`
	Region      string    `json:"region,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastRenewed time.Time `json:"last_renewed"`
}

func (c *Credential) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}
