-- +goose Up
-- SQL in section 'Up' is executed when this migration is applied.

create schema users;
alter schema users owner to rurdesk;

create schema projects;
alter schema projects owner to rurdesk;

create schema messages;
alter schema messages owner to rurdesk;

create schema issues;
alter schema issues owner to rurdesk;

create table users.user (
	id_user bigserial primary key,
	name character varying(250) not null,
	email character varying(250) not null unique,
	password character varying(250) not null,
	gravatar character varying(250),
	color_avatar_bg character varying(10) not null
);
alter table users.user owner to rurdesk;

create table users.team (
	id_team bigserial primary key,
	name character varying(250) not null
);
alter table users.team owner to rurdesk;

create table users.team_invitation (
	id_team 	bigint not null references users.team (id_team) ON DELETE CASCADE,
	uuid 		character varying(50) not null unique,
	email 		character varying(250) not null,
	accepted	boolean
);
alter table users.team_invitation owner to rurdesk;

create table users.user_team (
	id_user bigint not null references users.user (id_user) ON DELETE CASCADE,
	id_team bigint not null references users.team (id_team) ON DELETE CASCADE,
	primary key (id_user, id_team)
);
alter table users.user_team owner to rurdesk;

create table issues.state (
	id_state		bigserial not null primary key,
	name			character varying(20) not null,
	start			boolean not null,
	final			boolean not null,
	protected		boolean not null
);
alter table issues.state owner to rurdesk;

create table issues.severity (
	id_severity		bigserial not null primary key,
	title			character varying(20) not null,
	color			character varying(20) not null,
	protected		boolean not null
);
alter table issues.severity owner to rurdesk;

create table projects.project (
	id_project 				bigserial primary key,
	name 					character varying(250) not null,
	id_state_default 		bigint references issues.state (id_state) ON DELETE SET NULL,
	id_severity_default 	bigint references issues.severity (id_severity) ON DELETE SET NULL
);
alter table projects.project owner to rurdesk;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION projects.create_issue_sequence() RETURNS trigger
AS $$
DECLARE
	sequence_cmd text;
BEGIN 
	sequence_cmd := 'CREATE SEQUENCE issues.project_' || NEW.id_project || '_issue_public_seq';
	EXECUTE sequence_cmd;
	RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER tr_create_issue_sequence AFTER INSERT ON projects.project FOR EACH ROW EXECUTE FUNCTION projects.create_issue_sequence();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION projects.drop_issue_sequence() RETURNS trigger AS $$
	DECLARE
		sequence_cmd character varying(250);
    BEGIN
     	sequence_cmd := 'DROP SEQUENCE issues.project_' || OLD.id_project || '_issue_public_seq';
        EXECUTE sequence_cmd;
        RETURN NEW;
    END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER tr_drop_issue_sequence AFTER DELETE ON projects.project FOR EACH ROW EXECUTE FUNCTION projects.drop_issue_sequence();

create table projects.project_team (
	id_project 	bigint not null references projects.project (id_project) ON DELETE CASCADE,
	id_team 	bigint not null references users.team (id_team) ON DELETE CASCADE,
	PRIMARY KEY (id_project, id_team)
);
alter table projects.project_team owner to rurdesk;

create table projects.project_user (
	id_project 	bigint not null references projects.project (id_project) ON DELETE CASCADE,
	id_user 	bigint not null references users.user (id_user) ON DELETE CASCADE,
	PRIMARY KEY (id_project, id_user)
);
alter table projects.project_user owner to rurdesk;

CREATE TABLE projects.project_issue_state (
	id_project			BIGINT NOT NULL REFERENCES projects.project (id_project) ON DELETE CASCADE,
	id_state			BIGINT NOT NULL REFERENCES issues.state (id_state) ON DELETE CASCADE,
	order_rank			INT NOT NULL DEFAULT 0,
	PRIMARY KEY (id_project, id_state)
);
ALTER TABLE projects.project_issue_state OWNER TO rurdesk;

CREATE TABLE projects.project_issue_severity (
	id_project			BIGINT NOT NULL REFERENCES projects.project (id_project) ON DELETE CASCADE,
	id_severity			BIGINT NOT NULL REFERENCES issues.severity (id_severity) ON DELETE CASCADE,
	order_rank			INT NOT NULL DEFAULT 0,
	PRIMARY KEY (id_project, id_severity)
);
ALTER TABLE projects.project_issue_severity OWNER TO rurdesk;

create table issues.issue (
	id_issue 			bigserial not null primary key,
	id_issue_public		bigint not null,
	id_project			bigint not null references projects.project (id_project) ON DELETE CASCADE,
	id_state			bigint references issues.state (id_state) ON DELETE SET NULL,
	title				character varying (100) not null,
	description			text not null,
	create_at			TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT (now() at time zone 'utc'),
	update_at			TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT (now() at time zone 'utc'),
	create_by			BIGINT NOT NULL REFERENCES users.user (id_user) ON DELETE SET NULL,
	update_by			BIGINT NOT NULL REFERENCES users.user (id_user) ON DELETE SET NULL,
	assigned_to			BIGINT REFERENCES users.user (id_user) ON DELETE SET NULL,
	id_severity			BIGINT REFERENCES issues.severity (id_severity) ON DELETE SET NULL,
	tracked				BIGINT NOT NULL DEFAULT 0,
	estimated			BIGINT NOT NULL DEFAULT 0,
	scheduled_at 		TIMESTAMP WITHOUT TIME ZONE,
	UNIQUE (id_issue_public, id_project)
);
alter table issues.issue owner to rurdesk;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION issues.set_id_issue_public_from_seq() RETURNS trigger AS $$
	DECLARE
		sequence_cmd character varying(250);
    BEGIN
		NEW.id_issue_public := nextval(('issues.project_' || NEW.id_project || '_issue_public_seq')::regclass);
        RETURN NEW;
    END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER tr_set_id_issue_public_from_seq BEFORE INSERT ON issues.issue FOR EACH ROW EXECUTE FUNCTION issues.set_id_issue_public_from_seq();

create table messages.message (
	id_message 		bigserial primary key,
	message 		text not null,
	id_user_from 	bigint not null references users.user (id_user) ON DELETE CASCADE,
	created_at		timestamp without time zone not null default (now() at time zone 'utc')
);
alter table messages.message owner to rurdesk;

create table messages.user_message (
	id_message		bigint not null references messages.message(id_message) ON DELETE CASCADE,
	id_user_to 		bigint not null references users.user (id_user) ON DELETE CASCADE
);
alter table messages.user_message owner to rurdesk;

create table messages.project_message (
	id_message		bigint not null references messages.message(id_message) ON DELETE CASCADE,
	id_project_to 	bigint not null references projects.project (id_project) ON DELETE CASCADE
);
alter table messages.project_message owner to rurdesk;

create table messages.team_message (
	id_message		bigint not null references messages.message(id_message) ON DELETE CASCADE,
	id_team_to 		bigint not null references users.team (id_team) ON DELETE CASCADE
);
alter table messages.team_message owner to rurdesk;

create table messages.issue_message (
	id_message		bigint not null references messages.message(id_message) ON DELETE CASCADE,
	id_issue_to 	bigint not null references issues.issue (id_issue) ON DELETE CASCADE
);
alter table messages.issue_message owner to rurdesk;

create table messages.user_read (
	id_user 		bigint not null references users.user (id_user) ON DELETE CASCADE,
	id_user_from 	bigint not null references users.user (id_user) ON DELETE CASCADE,
	read_at 		timestamp without time zone not null default (now() at time zone 'utc')
);
alter table messages.user_read owner to rurdesk;

create table messages.project_read (
	id_user 		bigint not null references users.user (id_user) ON DELETE CASCADE,
	id_project_to 	bigint not null references projects.project (id_project) ON DELETE CASCADE,
	read_at			timestamp without time zone not null default (now() at time zone 'utc')
);
alter table messages.project_read owner to rurdesk;

create table messages.team_read (
	id_user 		bigint not null references users.user (id_user) ON DELETE CASCADE,
	id_team_to 		bigint not null references users.team (id_team) ON DELETE CASCADE,
	read_at			timestamp without time zone not null default (now() at time zone 'utc')
);
alter table messages.team_read owner to rurdesk;

CREATE TABLE issues.tracker (
	id_tracker 	BIGSERIAL NOT NULL PRIMARY KEY,
	id_user		BIGINT NOT NULL REFERENCES users.user (id_user) ON DELETE CASCADE,
	id_issue	BIGINT NOT NULL REFERENCES issues.issue (id_issue) ON DELETE CASCADE,
	start_at	TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT (now() at time zone 'utc'),
	UNIQUE (id_user)
);
ALTER TABLE issues.tracker OWNER TO rurdesk;

CREATE TABLE issues.track (
	id_track 	BIGSERIAL NOT NULL PRIMARY KEY,
	id_user		BIGINT NOT NULL REFERENCES users.user (id_user) ON DELETE CASCADE,
	id_issue	BIGINT NOT NULL REFERENCES issues.issue (id_issue) ON DELETE CASCADE,
	tracked		BIGINT NOT NULL,
	start_at	TIMESTAMP WITHOUT TIME ZONE,
	end_at		TIMESTAMP WITHOUT TIME ZONE
);
ALTER TABLE issues.track OWNER TO rurdesk;

CREATE TABLE issues.pin_destination_type (
	id_pin_destination_type 	INTEGER PRIMARY KEY,
	code 						CHARACTER VARYING (50) NOT NULL
);
ALTER TABLE issues.pin_destination_type OWNER TO rurdesk;

CREATE TABLE issues.pin (
	id_pin 						BIGSERIAL PRIMARY KEY,
	id_issue 					BIGINT NOT NULL REFERENCES issues.issue (id_issue) ON DELETE CASCADE,
	id_pin_destination			BIGINT NOT NULL,
	id_pin_destination_type 	BIGINT NOT NULL REFERENCES issues.pin_destination_type (id_pin_destination_type) ON DELETE CASCADE
);
ALTER TABLE issues.pin OWNER TO rurdesk;




INSERT INTO issues.state(name, start, final, protected) VALUES
('New', true, false, true),
('In progress', false, false, true),
('Closed', false, true, true);

INSERT INTO issues.severity(title, color, protected) VALUES
('Low', '#2196F3', true),
('Medium', '#FBC02D' , true),
('High', '#d32f2f', true);

INSERT INTO issues.pin_destination_type(id_pin_destination_type, code) VALUES
(1, 'user-page'),
(2, 'project-page');

-- +goose Down
-- SQL in section 'Down' is executed when this migration is rolled back.

drop schema messages cascade;
drop schema issues cascade;
drop schema projects cascade;
drop schema users cascade;
