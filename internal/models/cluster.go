package models

import (
	"time"
)

type Cluster struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Hosts     string    `json:"hosts"` // JSON array string
	AuthType  string    `json:"auth_type"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	APIKey    string    `json:"api_key"`
	Color     string    `json:"color"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
