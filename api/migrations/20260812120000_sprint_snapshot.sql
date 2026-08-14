-- +goose Up
CREATE TABLE issues.sprint_snapshot (
    id_sprint      bigint NOT NULL REFERENCES issues.sprint(id_sprint) ON DELETE CASCADE,
    day            date NOT NULL,
    total_points   int NOT NULL,
    done_points    int NOT NULL,
    total_issues   int NOT NULL,
    done_issues    int NOT NULL,
    pointed_issues int NOT NULL,
    PRIMARY KEY (id_sprint, day)
);

-- +goose Down
DROP TABLE issues.sprint_snapshot;
