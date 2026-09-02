-- name: CreateRefeshToken :one
INSERT INTO refresh_tokens (token, user_id, expires_at)
VALUES ($1, $2, $3)
RETURNING *;


-- name: GetUserFromRefreshToken :one
SELECT * FROM users
JOIN refresh_tokens ON users.id = refresh_tokens.user_id
WHERE refresh_tokens.token = $1;

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens
WHERE token = $1;

-- name: UpdateRefreshToken :one
UPDATE refresh_tokens
SET token = $2, updated_at = NOW(), expires_at = $3
WHERE user_id = $1
RETURNING *;


-- name: RevokeRefreshToken :one
UPDATE refresh_tokens
SET revoked_at = NOW()
WHERE token = $1
RETURNING *;