package repository

import "errors"

var (
	ErrUserNotFound = errors.New("user not found")

	ErrApiKeyNotFound = errors.New("api key not found")

	ErrRunNotFound        = errors.New("agent run not found")
	ErrPhaseMismatch      = errors.New("run phase does not match expected phase")
	ErrTaskNotFound       = errors.New("agent task not found")
	ErrTaskStatusMismatch = errors.New("agent task status does not match expected")

	ErrBotGatewayNotFound      = errors.New("bot gateway not found")
	ErrGitIntegrationDuplicate = errors.New("git integration duplicate")

	ErrAnchorWrongThread = errors.New("anchor parent message belongs to a different thread")
)
