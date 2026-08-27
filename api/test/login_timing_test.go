package test

import (
	"fmt"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A failed login must cost the same whether or not the account exists —
// returning early on an unknown email told an attacker who has an account here.
//
// The two paths are compared against each other rather than an absolute floor,
// because the hashing cost is configurable and any fixed threshold would be
// flaky or meaningless depending on it.
func TestLogin_UnknownEmailStillHashes(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)

	const password = "correct-horse"
	email := fmt.Sprintf("timing-%d@test.sk", time.Now().UnixNano())
	res := Request(t, app, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":"timing","email":%q,"password":%q}`, email, password), token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	fastestLogin := func(loginEmail string) time.Duration {
		fastest := time.Duration(math.MaxInt64)
		for i := 0; i < 5; i++ {
			start := time.Now()
			res := Request(t, app, "POST", "/api/public/login",
				fmt.Sprintf(`{"email":%q,"password":"definitely-wrong"}`, loginEmail), "")
			took := time.Since(start)
			require.Equal(t, http.StatusUnauthorized, res.StatusCode,
				"a wrong password must be rejected identically for %q", loginEmail)
			if took < fastest {
				fastest = took
			}
		}
		return fastest
	}

	existingTook := fastestLogin(email)
	unknownTook := fastestLogin("no-such-user@test.sk")

	require.Greater(t, unknownTook*8, existingTook,
		"unknown email returned in %v while an existing account took %v — hashing was "+
			"skipped, which leaks whether the account exists", unknownTook, existingTook)
}
