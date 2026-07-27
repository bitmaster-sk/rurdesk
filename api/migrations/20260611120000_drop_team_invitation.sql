-- +goose Up
-- Invitations are removed: admin manages users/teams directly.
-- Old invitation notifications would be unrenderable in the client.
DELETE FROM notification.notification WHERE type = 'invitation';
DROP TABLE IF EXISTS users.team_invitation;

-- +goose Down
create table users.team_invitation (
	id_team 	bigint not null references users.team (id_team) ON DELETE CASCADE,
	uuid 		character varying(50) not null unique,
	email 		character varying(250) not null,
	accepted	boolean
);
alter table users.team_invitation OWNER TO rurdesk;
