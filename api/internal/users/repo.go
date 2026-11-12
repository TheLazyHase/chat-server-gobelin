package users

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"chat-gobelin-api/internal"
	"github.com/jmoiron/sqlx"
)

type Repo interface {
	GetByUserName(ctx context.Context, username string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	Create(ctx context.Context, name string, metadata json.RawMessage, passwordHash []byte) (*User, error)
	EditMetadata(ctx context.Context, user User, rawMetadata json.RawMessage) (*User, error)
	GetByIDs(ctx context.Context, userIDs []int64) ([]User, error)
	GetAll(ctx context.Context) ([]User, error)
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

	metadata := make(map[string]json.RawMessage)
	//This will catch null value, [] who were introduced as a bug, and useless {}
	if len(user.RawMetadata) > 2 {
		err = json.Unmarshal(user.RawMetadata, &metadata)
		if err != nil {
			return &user, fmt.Errorf("Error unmarshaling metadata: %w", err)
		}
	}
	user.Metadata = metadata

	return &user, nil
}

func (r *repo) GetByID(ctx context.Context, id int64) (*User, error) {
	query := `SELECT
			id,
			username,
			metadata
		FROM users
		WHERE id = ?`

	query = r.db.Rebind(query)
	var user User
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		return &user, fmt.Errorf("Error retrieving user by id: %w", err)
	}

	metadata := make(map[string]json.RawMessage)
	//This will catch null value, [] who were introduced as a bug, and useless {}
	if len(user.RawMetadata) > 2 {
		err = json.Unmarshal(user.RawMetadata, &metadata)
		if err != nil {
			return &user, fmt.Errorf("Error unmarshaling metadata: %w", err)
		}
	}
	user.Metadata = metadata

	return &user, nil
}

func (r *repo) GetByIDs(ctx context.Context, userIDs []int64) ([]User, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	//We don't retrieve balance nor passwords (or hashes) when retrieving multiple users
	query, args, err := sqlx.In(`SELECT
			id,
			username,
			metadata
		FROM users
		WHERE id IN (?)`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("Error hydrating query: %w", err)
	}
	query = r.db.Rebind(query)

	var users []User
	err = r.db.SelectContext(ctx, &users, query, args...)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving users: %w", err)
	}
	for i, user := range users {
		metadata := make(map[string]json.RawMessage)
		log.Printf("Error retrieving users: %s", user.RawMetadata)
		//This will catch null value, [] who were introduced as a bug, and useless {}
		if len(user.RawMetadata) > 2 {
			err = json.Unmarshal(user.RawMetadata, &metadata)
			if err != nil {
				return users, fmt.Errorf("Error unmarshaling metadata: %w", err)
			}
		}
		users[i].Metadata = metadata
	}

	return users, nil
}

func (r *repo) GetAll(ctx context.Context) ([]User, error) {
	//We don't retrieve balance nor passwords (or hashes) when retrieving multiple users
	query := `SELECT
			id,
			username,
			metadata
		FROM users`
	query = r.db.Rebind(query)

	var users []User
	err := r.db.SelectContext(ctx, &users, query)
	if err != nil {
		return nil, fmt.Errorf("Error retrieving users: %w", err)
	}
	for i, user := range users {
		metadata := make(map[string]json.RawMessage)
		log.Printf("Error retrieving users: %s", user.RawMetadata)
		//This will catch null value, [] who were introduced as a bug, and useless {}
		if len(user.RawMetadata) > 2 {
			err = json.Unmarshal(user.RawMetadata, &metadata)
			if err != nil {
				return users, fmt.Errorf("Error unmarshaling metadata: %w", err)
			}
		}
		users[i].Metadata = metadata
	}

	return users, nil
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
	user.Operation = &internal.USER_CREATED

	return &user, nil
}

func (r *repo) EditMetadata(ctx context.Context, user User, rawMetadata json.RawMessage) (*User, error) {

	var metadata map[string]json.RawMessage
	err := json.Unmarshal(rawMetadata, &metadata)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshaling new metadata for user: %w", err)
	}
	query := `UPDATE users SET
		metadata = ?
		WHERE id = ?
		RETURNING id, username, metadata`

	query = r.db.Rebind(query)
	var editedUser User
	err = r.db.GetContext(ctx, &editedUser, query, []byte(rawMetadata), user.ID)
	if err != nil {
		return nil, fmt.Errorf("Error creating user: %w", err)
	}

	editedUser.Metadata = metadata
	editedUser.Operation = &internal.USER_EDITED

	return &editedUser, nil
}
