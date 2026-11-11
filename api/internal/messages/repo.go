package messages

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"chat-gobelin-api/internal"
	"chat-gobelin-api/internal/users"
	"chat-gobelin-api/internal/channels"

	"github.com/jmoiron/sqlx"
)

type Repo interface {
	GetByChannel(ctx context.Context, channel channels.Channel) ([]Message, error)
	GetByID(ctx context.Context, id int64) (*Message, error)
	Create(ctx context.Context, content string, rawMetadata json.RawMessage, channel channels.Channel, user users.User, anonymous bool, createdAt *time.Time) (*Message, error)
	Edit(ctx context.Context, message Message, content string, rawMetadata json.RawMessage) (*Message, error)
}

type repo struct {
	db *sqlx.DB
}

func NewRepo(db *sqlx.DB) (Repo, error) {
	return &repo{
		db: db,
	}, nil
}


func (r *repo) GetByChannel(ctx context.Context, channel channels.Channel) ([]Message, error) {
	query := `SELECT
			a.id,
			a.channel_id,
			a.content,
			a.user_id,
			a.creation_date,
			a.metadata,
			a.anonymous,
			a.posted_at,
			b.username as user
		FROM messages a
		JOIN users b ON a.user_id = b.id
		WHERE a.channel_id = ? AND posted_at < NOW()
		ORDER BY posted_at DESC
		LIMIT 100`

	query = r.db.Rebind(query)
	var messages []Message
	err := r.db.SelectContext(ctx, &messages, query, channel.ID)
	if err != nil {
		return messages, fmt.Errorf("Error retrieving channels: %w", err)
	}

	for i, message := range messages {
		metadata, err := internal.DecodeMetadata(message.RawMetadata)
		if err != nil {
			return messages, fmt.Errorf("Error unmarshaling metadata: %w for %s", err, message.RawMetadata)
		}
		messages[i].Metadata = metadata
	}

	return messages, nil
}

func (r *repo) GetByID(ctx context.Context, id int64) (*Message, error) {
	query := `SELECT
			a.id,
			a.channel_id,
			a.content,
			a.user_id,
			a.creation_date,
			a.metadata,
			a.anonymous,
			a.posted_at,
			b.username as user
		FROM messages a
		JOIN users b ON a.user_id = b.id
		WHERE a.id = ?`

	query = r.db.Rebind(query)
	var message Message
	err := r.db.GetContext(ctx, &message, query, id)
	if err != nil {
		return &message, fmt.Errorf("Error retrieving message by id: %w", err)
	}

	metadata, err := internal.DecodeMetadata(message.RawMetadata)
	if err != nil {
		return &message, fmt.Errorf("Error unmarshaling metadata: %w", err)
	}
	message.Metadata = metadata

	return &message, nil
}

func (r *repo) Create(ctx context.Context, content string, rawMetadata json.RawMessage, channel channels.Channel, user users.User, anonymous bool, createdAt *time.Time) (*Message, error) {
	//The column is NOT NULL, a nil value would be inserted as NULL
	if rawMetadata == nil {
		rawMetadata = json.RawMessage(`{}`)
	}

	var message Message
	if createdAt == nil {
		query := `INSERT INTO messages
			(content, metadata, user_id, channel_id, anonymous, posted_at, creation_date) VALUES (?, ?, ?, ?, ?, NOW(), NOW())
			RETURNING id, channel_id, content, user_id, anonymous, posted_at, creation_date`

		query = r.db.Rebind(query)
		err := r.db.GetContext(ctx, &message, query, content, rawMetadata, user.ID, channel.ID, anonymous)
		if err != nil {
			return nil, fmt.Errorf("Error creating channel: %w", err)
		}
	} else {
		query := `INSERT INTO messages
			(content, metadata, user_id, channel_id, anonymous, posted_at, creation_date) VALUES (?, ?, ?, ?, ?, ?, NOW())
			RETURNING id, channel_id, content, user_id, anonymous, posted_at, creation_date`

		query = r.db.Rebind(query)
		err := r.db.GetContext(ctx, &message, query, content, rawMetadata, user.ID, channel.ID, anonymous, createdAt)
		if err != nil {
			return nil, fmt.Errorf("Error creating channel: %w", err)
		}
	}
	message.User = user.Username
	message.RawMetadata = rawMetadata
	return &message, nil
}

func (r *repo) Edit(ctx context.Context, message Message, content string, rawMetadata json.RawMessage) (*Message, error) {
	//Idempotent if nothing changed
	if content == "" && rawMetadata == nil {
		return &message, nil
	}

	//An empty content or a nil metadata keeps the current value
	query := `UPDATE messages SET
		content = COALESCE(NULLIF(?, ''), content),
		metadata = COALESCE(?, metadata)
		WHERE id = ?`
	query = r.db.Rebind(query)
	_, err := r.db.ExecContext(ctx, query, content, []byte(rawMetadata), message.ID)
	if err != nil {
		return nil, fmt.Errorf("Error updating message: %w", err)
	}

	//Read back the whole message, the update only knows the edited columns
	return r.GetByID(ctx, message.ID)
}
