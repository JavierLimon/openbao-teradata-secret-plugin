package models

import "time"

type Credential struct {
	Username    string    `json:"username"`
	RoleName    string    `json:"role_name"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastRenewed time.Time `json:"last_renewed"`
}
