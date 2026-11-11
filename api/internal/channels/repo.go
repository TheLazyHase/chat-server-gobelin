package channels

import (
	"context"
	"encoding/json"
	"fmt"

	"chat-gobelin-api/internal/users"

	"github.com/jmoiron/sqlx"
)

type Repo interface {
	GetByUser(ctx context.Context, user users.User) ([]Channel, error)
	GetByID(ctx context.Context, id int64) (*Channel, error)
	GetByName(ctx context.Context, name string) (*Channel, error)
	GetAllChannels(ctx context.Context) ([]Channel, error)
	Create(ctx context.Context, name string, metadata json.RawMessage) (*Channel, error)
	AddUserToChannel(ctx context.Context, channel Channel, user users.User) error
	Edit(ctx context.Context, channel Channel, content string, rawMetadata json.RawMessage) (*Channel, error)
}

type repo struct {
	db *sqlx.DB
}

func NewRepo(db *sqlx.DB) (Repo, error) {
	return &repo{
		db: db,
	}, nil
}

func (r *repo) GetByUser(ctx context.Context, user users.User) ([]Channel, error) {
	query := `SELECT
			a.id,
			a.name,
			a.metadata,
			a.creation_date
		FROM channels a
		JOIN user_channels b ON a.id = b.channel_id
		WHERE b.user_id = ?`

	query = r.db.Rebind(query)
	var channels []Channel
	err := r.db.SelectContext(ctx, &channels, query, user.ID)
	if err != nil {
		return channels, fmt.Errorf("Error retrieving channels: %w", err)
	}
	return channels, nil
}

func (r *repo) GetAllChannels(ctx context.Context) ([]Channel, error) {
	query := `SELECT
			id,
			name,
			metadata
		FROM channels`

	query = r.db.Rebind(query)
	var channels []Channel
	err := r.db.SelectContext(ctx, &channels, query)
	if err != nil {
		return channels, fmt.Errorf("Error retrieving channels: %w", err)
	}
	return channels, nil
}

func (r *repo) GetByName(ctx context.Context, name string) (*Channel, error) {
	query := `SELECT
			id,
			name,
			metadata,
			creation_date
		FROM channels
		WHERE name = ?`

	query = r.db.Rebind(query)
	var channel Channel
	err := r.db.GetContext(ctx, &channel, query, name)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving channels: %w", err)
	}
	return &channel, nil
}

func (r *repo) GetByID(ctx context.Context, id int64) (*Channel, error) {
	query := `SELECT
			id,
			name,
			metadata,
			creation_date
		FROM channels
		WHERE id = ?`

	query = r.db.Rebind(query)
	var channel Channel
	err := r.db.GetContext(ctx, &channel, query, id)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving channels: %w", err)
	}
	return &channel, nil
}

func (r *repo) Create(ctx context.Context, name string, metadata json.RawMessage) (*Channel, error) {
	query := `INSERT INTO channels
		(name, metadata, creation_date) VALUES (?, ?, NOW())
		RETURNING id, name, metadata, creation_date`

	//The column is NOT NULL, a nil value would be inserted as NULL
	if metadata == nil {
		metadata = json.RawMessage(`{}`)
	}

	query = r.db.Rebind(query)
	var channel Channel
	err := r.db.GetContext(ctx, &channel, query, name, metadata)
	if err != nil {
		return nil, fmt.Errorf("Error creating channel: %w", err)
	}

	//TODO: add creator to it
	return &channel, nil
}

func (r *repo) AddUserToChannel(ctx context.Context, channel Channel, user users.User) error {
	query := `INSERT INTO user_channels
		(user_id, channel_id) VALUES (?, ?)
		RETURNING channel_id as id`

	query = r.db.Rebind(query)
	var added Channel
	err := r.db.GetContext(ctx, &added, query, user.ID, channel.ID)
	if err != nil {
		return fmt.Errorf("Error creating channel: %w", err)
	}
	return nil
}

func (r *repo) Edit(ctx context.Context, channel Channel, content string, rawMetadata json.RawMessage) (*Channel, error) {
	var editedChannel Channel

	query := `UPDATE channel SET
		name = ?,
		metadata = ?
		WHERE id = ?
		RETURNING id, name, metadata`
	query = r.db.Rebind(query)
	err := r.db.GetContext(ctx, &editedChannel, query, content, []byte(rawMetadata), channel.ID)
	if err != nil {
		return nil, fmt.Errorf("Error updating channel: %w", err)
	}

	var metadata map[string]json.RawMessage
	err = json.Unmarshal(rawMetadata, &metadata)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshaling edited metadata for user: %w", err)
	}
	editedChannel.Metadata = metadata

	return &editedChannel, nil
}
