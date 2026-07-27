package test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListUsers_RequiresAuth(t *testing.T) {
	app := Setup(t)
	res := Request(t, app, "GET", "/api/private/users", "", "")
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}

func Test_ListUsers_ReturnsAllUsersWithoutPasswords(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)

	res := Request(t, app, "GET", "/api/private/users", "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	body := readBody(t, res)
	assert.False(t, strings.Contains(body, "password"), "must not leak password hashes")

	var users []model.User
	require.Nil(t, json.Unmarshal([]byte(body), &users))
	assert.GreaterOrEqual(t, len(users), 1) // at least the bootstrap admin
}
