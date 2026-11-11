package messages

import (
	"encoding/json"
	"time"
)

type Message struct {
	ID           int64  `json:"id"`
	ChannelID    int64  `db:"channel_id" json:"channel_id"`
	Content      string `json:"content"`
	UserID       int64  `db:"user_id" json:"user_id"`
	User         string `json:"user"`
	Anonymous    bool   `db:"anonymous" json:"anonymous"`
	CreationDate string `db:"creation_date" json:"creation_date"`
	PostedAt     string `db:"posted_at" json:"posted_at"`
	RawMetadata  []byte  `db:"metadata" json:"-"`
	Metadata     map[string]json.RawMessage  `db:"-" json:"metadata"`
}

//False until the posted_at of a scheduled message is reached
func (m *Message) IsPosted() bool {
	postedAt, err := time.Parse(time.RFC3339Nano, m.PostedAt)
	if err != nil {
		return true
	}
	return !postedAt.After(time.Now())
}

//Hides the author of an anonymous message, to call before sending it to clients
func (m *Message) Anonymize() {
	if m.Anonymous {
		m.User = "anonymous"
		m.UserID = 0
	}
}
