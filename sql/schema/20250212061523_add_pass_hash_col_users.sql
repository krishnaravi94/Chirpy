-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD hashed_password TEXT NOT NULL DEFAULT 'unset'
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER users DROP hashed_password
-- +goose StatementEnd
