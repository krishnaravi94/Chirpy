-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email,hashed_password)
VALUES (
    gen_random_uuid(),NOW(),NOW(),$1,$2
)
RETURNING *;

-- name: CheckUser :one
SELECT * FROM users where id=$1;

-- name: CheckUserWithEmail :one
SELECT * FROM users where email=$1;