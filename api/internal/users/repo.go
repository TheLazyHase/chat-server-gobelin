package users

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Repo interface {
	GetByUserName(ctx context.Context, username string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	Create(ctx context.Context, name string, metadata json.RawMessage, passwordHash []byte) (*User, error)
}

type repo struct {
	db *sqlx.DB
}

func NewRepo(db *sqlx.DB) (Repo, error) {
	return &repo{
		db: db,
	}, nil
}

func (r *repo) GetByUserName(ctx context.Context, username string) (*User, error) {
	query := `SELECT
			id,
			username,
			password_hash,
			metadata
		FROM users
		WHERE username = ?`

	query = r.db.Rebind(query)
	var user User
	err := r.db.GetContext(ctx, &user, query, username)
	if err != nil {
		return &user, fmt.Errorf("Error retrieving users by username: %w", err)
	}

	var metadata map[string]json.RawMessage
	err = json.Unmarshal(user.RawMetadata, &metadata)
	if err != nil {
		return &user, fmt.Errorf("Error unmarshaling metadata: %w", err)
	}
	user.Metadata = metadata

	return &user, nil
}

func (r *repo) GetByID(ctx context.Context, id int64) (*User, error) {
	query := `SELECT
			id,
			username
		FROM users
		WHERE id = ?`

	query = r.db.Rebind(query)
	var user User
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		return &user, fmt.Errorf("Error retrieving user by id: %w", err)
	}
	return &user, nil
}

func (r *repo) Create(ctx context.Context, name string, rawMetadata json.RawMessage, passwordHash []byte) (*User, error) {
	var metadata map[string]json.RawMessage
	err := json.Unmarshal(rawMetadata, &metadata)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshaling metadata for new user: %w", err)
	}
	query := `INSERT INTO users
		(username, password_hash, metadata, creation_date) VALUES (?, ?, ?, NOW())
		RETURNING id, username, metadata`

	query = r.db.Rebind(query)
	var user User
	err = r.db.GetContext(ctx, &user, query, name, passwordHash, []byte(rawMetadata))
	if err != nil {
		return nil, fmt.Errorf("Error creating user: %w", err)
	}
	user.Metadata = metadata
	return &user, nil
}
