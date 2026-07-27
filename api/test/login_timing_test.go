package test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// A failed login must cost the same whether or not the account exists.
//
// Returning early on an unknown email skipped bcrypt entirely. bcrypt at
// DefaultCost takes tens of milliseconds while the early return took
// microseconds, so the response time told an attacker which addresses have an
// account here — on a self-hosted company tracker, that is the staff list.
//
// The assertion is a floor rather than a comparison of the two timings: the
// no-such-user path simply has to do real hashing work. The margin between
// "bcrypt ran" and "we returned immediately" is three orders of magnitude, so a
// 10ms floor is far below anything bcrypt produces and far above the early
// return, and does not turn into a flaky benchmark on a loaded CI box.
func TestLogin_UnknownEmailStillHashes(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)

	const password = "correct-horse"
	email := fmt.Sprintf("timing-%d@test.sk", time.Now().UnixNano())
	res := Request(t, app, "POST", "/api/private/admin/user",
		fmt.Sprintf(`{"name":"timing","email":%q,"password":%q}`, email, password), token)
	require.Equal(t, http.StatusOK, res.StatusCode)

	timeLogin := func(loginEmail string) (int, time.Duration) {
		start := time.Now()
		res := Request(t, app, "POST", "/api/public/login",
			fmt.Sprintf(`{"email":%q,"password":"definitely-wrong"}`, loginEmail), "")
		return res.StatusCode, time.Since(start)
	}

	existingStatus, existingTook := timeLogin(email)
	unknownStatus, unknownTook := timeLogin("no-such-user@test.sk")

	require.Equal(t, http.StatusUnauthorized, existingStatus, "wrong password must be rejected")
	require.Equal(t, http.StatusUnauthorized, unknownStatus, "unknown email must look identical")

	require.Greater(t, unknownTook, 10*time.Millisecond,
		"unknown email returned in %v (existing account took %v) — bcrypt was skipped, "+
			"which leaks whether the account exists", unknownTook, existingTook)
}
