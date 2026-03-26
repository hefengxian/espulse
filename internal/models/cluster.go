package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// StringArray is a slice of strings that can be saved to/loaded from SQLite as a JSON string
type StringArray []string

// Scan implements the sql.Scanner interface
func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return errors.New("invalid type for StringArray")
	}
	return json.Unmarshal([]byte(s), a)
}

// Value implements the driver.Valuer interface
func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	res, err := json.Marshal(a)
	return string(res), err
}

type Cluster struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Hosts     StringArray `json:"hosts"` // JSON array in API and DB
	AuthType  string      `json:"auth_type"`
	Username  string      `json:"username"`
	Password  string      `json:"password"`
	APIKey    string      `json:"api_key"`
	Color     string      `json:"color"`
	Notes     string      `json:"notes"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}
