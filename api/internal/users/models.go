package users

import "encoding/json"

type User struct {
	Username     string `json:"username"`
	PasswordHash []byte `json:"-" db:"password_hash"` // We never show the password hash
	ID           int64  `json:"id"`
	RawMetadata  []byte  `db:"metadata" json:"-"`
	Metadata     map[string]json.RawMessage  `db:"-" json:"metadata"`
}
