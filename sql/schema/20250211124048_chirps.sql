-- +goose Up
CREATE TABLE chirps(
    id UUID primary key, 
    created_at timestamp NOT NULL, 
    updated_at timestamp NOT NULL, 
    body text NOT NULL,
    user_id UUID NOT NULL references users (id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE chirps;
