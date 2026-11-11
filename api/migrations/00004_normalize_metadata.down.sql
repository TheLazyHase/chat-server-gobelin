ALTER TABLE users
    ALTER COLUMN metadata SET DEFAULT '';

ALTER TABLE channels
    ALTER COLUMN metadata SET DEFAULT '';

ALTER TABLE messages
    ALTER COLUMN metadata SET DEFAULT '';
