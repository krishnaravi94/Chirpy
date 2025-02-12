-- name: CreateChirp :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (
    gen_random_uuid(),NOW(),NOW(),$1,$2
)
RETURNING *;

-- name: GetChirpsByCreatedAt :many
SELECT * FROM chirps order by created_at asc;

-- name: GetChirpByID :one
SELECT * FROM chirps where id=$1;