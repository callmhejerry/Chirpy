-- name: CreateUser :one
INSERT INTO users (email)
VALUES ($1)
RETURNING *;


-- name: DeleteAllUsers :exec
DELETE FROM users;


-- name: GetUserById :one
SELECT * FROM users
WHERE id = $1;