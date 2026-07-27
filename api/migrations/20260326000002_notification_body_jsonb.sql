-- +goose Up
ALTER TABLE notification.notification
    ALTER COLUMN body TYPE jsonb
    USING CASE
        WHEN body IS NULL OR body = '' THEN NULL
        ELSE body::jsonb
    END;

-- +goose Down
ALTER TABLE notification.notification
    ALTER COLUMN body TYPE text
    USING body::text;
