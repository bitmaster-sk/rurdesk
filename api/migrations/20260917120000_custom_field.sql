-- +goose Up
CREATE TABLE issues.custom_field (
    id_custom_field bigserial PRIMARY KEY,
    id_project      bigint NOT NULL REFERENCES projects.project (id_project) ON DELETE CASCADE,
    key             character varying(40) NOT NULL,
    name            character varying(60) NOT NULL,
    field_type      text NOT NULL,
    is_required     boolean NOT NULL DEFAULT false,
    required_since  timestamp,
    default_value   jsonb,
    order_rank      int NOT NULL DEFAULT 0,
    archived_at     timestamp,
    CONSTRAINT custom_field_type_chk CHECK (field_type IN ('text', 'number', 'date', 'select', 'boolean')),
    CONSTRAINT custom_field_key_chk CHECK (key ~ '^[a-z][a-z0-9_]*$'),
    CONSTRAINT custom_field_key_uq UNIQUE (id_project, key),
    CONSTRAINT custom_field_id_type_uq UNIQUE (id_custom_field, field_type)
);
ALTER TABLE issues.custom_field OWNER TO rurdesk;

CREATE INDEX idx_custom_field_project_order
    ON issues.custom_field (id_project, order_rank)
    WHERE archived_at IS NULL;

CREATE TABLE issues.custom_field_option (
    id_option       bigserial PRIMARY KEY,
    id_custom_field bigint NOT NULL REFERENCES issues.custom_field (id_custom_field) ON DELETE CASCADE,
    label           character varying(60) NOT NULL,
    order_rank      int NOT NULL DEFAULT 0,
    CONSTRAINT custom_field_option_field_uq UNIQUE (id_custom_field, id_option)
);
ALTER TABLE issues.custom_field_option OWNER TO rurdesk;

CREATE INDEX idx_custom_field_option_field ON issues.custom_field_option (id_custom_field, order_rank);

CREATE TABLE issues.issue_custom_value (
    id_issue        bigint NOT NULL REFERENCES issues.issue (id_issue) ON DELETE CASCADE,
    id_custom_field bigint NOT NULL,
    field_type      text NOT NULL,
    value_text      text,
    value_number    numeric,
    value_date      timestamp,
    value_bool      boolean,
    id_option       bigint,
    PRIMARY KEY (id_issue, id_custom_field),
    CONSTRAINT issue_custom_value_field_fk
        FOREIGN KEY (id_custom_field, field_type)
        REFERENCES issues.custom_field (id_custom_field, field_type) ON DELETE CASCADE,
    CONSTRAINT issue_custom_value_option_fk
        FOREIGN KEY (id_custom_field, id_option)
        REFERENCES issues.custom_field_option (id_custom_field, id_option) ON DELETE RESTRICT,
    CONSTRAINT issue_custom_value_shape_chk CHECK (
        CASE field_type
            WHEN 'text'    THEN value_text   IS NOT NULL AND value_number IS NULL AND value_date IS NULL AND value_bool IS NULL AND id_option IS NULL
            WHEN 'number'  THEN value_number IS NOT NULL AND value_text   IS NULL AND value_date IS NULL AND value_bool IS NULL AND id_option IS NULL
            WHEN 'date'    THEN value_date   IS NOT NULL AND value_text   IS NULL AND value_number IS NULL AND value_bool IS NULL AND id_option IS NULL
            WHEN 'boolean' THEN value_bool   IS NOT NULL AND value_text   IS NULL AND value_number IS NULL AND value_date IS NULL AND id_option IS NULL
            WHEN 'select'  THEN id_option    IS NOT NULL AND value_text   IS NULL AND value_number IS NULL AND value_date IS NULL AND value_bool IS NULL
            ELSE false
        END
    )
);
ALTER TABLE issues.issue_custom_value OWNER TO rurdesk;

CREATE INDEX idx_issue_custom_value_field ON issues.issue_custom_value (id_custom_field);

-- +goose Down
DROP TABLE issues.issue_custom_value;
DROP TABLE issues.custom_field_option;
DROP TABLE issues.custom_field;
