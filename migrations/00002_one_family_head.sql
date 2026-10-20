-- +goose Up
-- Keep a single head if a race already created two, then enforce it.
UPDATE users
SET is_family_head = FALSE
WHERE id IN (
    SELECT id FROM (
        SELECT id FROM users WHERE is_family_head = TRUE ORDER BY id OFFSET 1
    ) extra
);

CREATE UNIQUE INDEX uniq_users_one_family_head
    ON users (is_family_head)
    WHERE is_family_head = TRUE;

-- +goose Down
DROP INDEX IF EXISTS uniq_users_one_family_head;
