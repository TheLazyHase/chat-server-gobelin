UPDATE users SET metadata = '{}' WHERE metadata IN ('', '[]');

UPDATE channels SET metadata = '{}' WHERE metadata IN ('', '[]');

UPDATE messages SET metadata = '{}' WHERE metadata IN ('', '[]');

ALTER TABLE users
    ALTER COLUMN metadata SET DEFAULT '{}';

ALTER TABLE channels
    ALTER COLUMN metadata SET DEFAULT '{}';

ALTER TABLE messages
    ALTER COLUMN metadata SET DEFAULT '{}';
