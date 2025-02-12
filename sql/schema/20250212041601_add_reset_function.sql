-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reset_database() RETURNS void AS $$
BEGIN
    TRUNCATE TABLE users, chirps CASCADE;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS reset_database();