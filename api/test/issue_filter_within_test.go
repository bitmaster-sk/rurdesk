package test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/stretchr/testify/require"
)

// Rolling windows are resolved server-side and pinned in the keyset cursor.

func withinURL(idProject int64, query string) string {
	return fmt.Sprintf(
		"/api/private/project/%d/issue?%s&stateUnset=false&severityUnset=false&assignedToUnset=false",
		idProject, query,
	)
}

// Direct SQL because no endpoint can set update_at — the write path owns it.
func backdateIssue(t *testing.T, app *issue.Application, idIssuePublic int64, age time.Duration) {
	_, err := app.Pool.Exec(t.Context(),
		`UPDATE issues.issue SET update_at = (now() at time zone 'utc') - $1::interval
		 WHERE id_issue_public = $2`,
		fmt.Sprintf("%d seconds", int64(age.Seconds())), idIssuePublic)
	require.Nil(t, err)
}

func TestGetIssues_UpdateAtWithinExcludesOlderIssues(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "within-basic")

	fresh := createIssue(t, app, token, idProject, "touched now")
	stale := createIssue(t, app, token, idProject, "touched long ago")
	backdateIssue(t, app, stale.IdIssuePublic, 10*24*time.Hour)

	page := loadIssuesPage(t, app, token, withinURL(idProject, "updateAtWithin=2d"))
	require.ElementsMatch(t, []int64{fresh.IdIssuePublic}, publicIds(page))

	// a window wide enough to cover both
	page = loadIssuesPage(t, app, token, withinURL(idProject, "updateAtWithin=30d"))
	require.Len(t, page.Items, 2)
}

// Why the wire format carries a unit: 1d8h6m is not a whole number of days.
func TestGetIssues_WithinAcceptsCombinedAndSubDayUnits(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "within-units")

	recent := createIssue(t, app, token, idProject, "an hour old")
	older := createIssue(t, app, token, idProject, "two days old")
	backdateIssue(t, app, recent.IdIssuePublic, 1*time.Hour)
	backdateIssue(t, app, older.IdIssuePublic, 48*time.Hour)

	// sub-day unit excludes the 2-day-old issue
	page := loadIssuesPage(t, app, token, withinURL(idProject, "updateAtWithin=3h"))
	require.ElementsMatch(t, []int64{recent.IdIssuePublic}, publicIds(page))

	// combined: 1d8h6m = 32h6m, still short of 48h
	page = loadIssuesPage(t, app, token, withinURL(idProject, "updateAtWithin=1d8h6m"))
	require.ElementsMatch(t, []int64{recent.IdIssuePublic}, publicIds(page))

	// 2d1h clears 48h
	page = loadIssuesPage(t, app, token, withinURL(idProject, "updateAtWithin=2d1h"))
	require.Len(t, page.Items, 2)
}

func TestGetIssues_CreateAtWithinFiltersOnCreation(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "within-create")

	createIssue(t, app, token, idProject, "created now")

	require.Len(t, loadIssuesPage(t, app, token, withinURL(idProject, "createAtWithin=1h")).Items, 1)
	require.Len(t, loadIssuesPage(t, app, token, withinURL(idProject, "createAtWithin=1s")).Items, 1)
}

// Refused, not ignored: dropping the window would return more than asked for.
func TestGetIssues_MalformedWithinIs422(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "within-invalid")
	createIssue(t, app, token, idProject, "an issue")

	for _, raw := range []string{"30x", "abc", "30", "-30m", "0d", "1M", "2w"} {
		t.Run(raw, func(t *testing.T) {
			res := Request(t, app, "GET", withinURL(idProject, "updateAtWithin="+raw), "", token)
			require.Equal(t, http.StatusUnprocessableEntity, res.StatusCode,
				"a window that cannot be parsed must not silently widen the result set")
			var body struct {
				Code string `json:"code"`
			}
			require.Nil(t, json.NewDecoder(res.Body).Decode(&body))
			require.Equal(t, "INVALID_WITHIN", body.Code)
		})
	}
}

// A window replaces only the lower bound — a co-supplied *AtTo still caps the top.
func TestGetIssues_WithinIntersectsWithUpdateAtTo(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "within-intersect")

	createIssue(t, app, token, idProject, "updated now")
	mid := createIssue(t, app, token, idProject, "updated three days ago")
	old := createIssue(t, app, token, idProject, "updated forty days ago")
	backdateIssue(t, app, mid.IdIssuePublic, 3*24*time.Hour)
	backdateIssue(t, app, old.IdIssuePublic, 40*24*time.Hour)

	upTo := url.QueryEscape(time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339))
	page := loadIssuesPage(t, app, token,
		withinURL(idProject, "updateAtWithin=30d&updateAtTo="+upTo))
	require.ElementsMatch(t, []int64{mid.IdIssuePublic}, publicIds(page))
}

func TestGetIssues_GroupedCursorCarriesWithinRef(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "within-grouped")

	createIssue(t, app, token, idProject, "first")
	createIssue(t, app, token, idProject, "second")

	res := Request(t, app, "GET",
		withinURL(idProject, "updateAtWithin=30d&limit=1&groupBy=state"), "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var body struct {
		Groups []struct {
			NextCursor *string `json:"nextCursor"`
		} `json:"groups"`
	}
	require.Nil(t, json.NewDecoder(res.Body).Decode(&body))
	require.Len(t, body.Groups, 1)
	require.NotNil(t, body.Groups[0].NextCursor)
	require.NotEmpty(t, cursorRef(t, *body.Groups[0].NextCursor))
}

// The window must stay fixed across a paged traversal, or the lower bound creeps.
func TestGetIssues_WithinReferenceInstantIsPinnedInCursor(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "within-cursor")

	created := []int64{}
	for i := 0; i < 3; i++ {
		created = append(created, createIssue(t, app, token, idProject,
			fmt.Sprintf("pinned %d", i)).IdIssuePublic)
	}

	first := loadIssuesPage(t, app, token, withinURL(idProject, "updateAtWithin=30d&limit=2"))
	require.Len(t, first.Items, 2)
	require.NotNil(t, first.NextCursor, "three issues over a page size of two must yield a cursor")
	require.NotEmpty(t, cursorRef(t, *first.NextCursor),
		"a rolling window must pin its reference instant in the cursor")

	second := loadIssuesPage(t, app, token,
		withinURL(idProject, "updateAtWithin=30d&limit=2&cursor="+*first.NextCursor))
	require.Len(t, second.Items, 1)

	// every row seen exactly once — no skips, no repeats
	require.ElementsMatch(t, created, append(publicIds(first), publicIds(second)...))
}

// Proves the pin is USED, not merely carried: anchored 35 days back, a 30d window
// reaches 65 days and picks up a row a fresh now() would exclude.
func TestGetIssues_WithinResolvesAgainstTheCursorsReference(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "within-cursor-honoured")

	// three rows inside the window, so page 1 leaves a cursor behind
	for i := 0; i < 3; i++ {
		createIssue(t, app, token, idProject, fmt.Sprintf("updated today %d", i))
	}
	old := createIssue(t, app, token, idProject, "updated 40 days ago")
	backdateIssue(t, app, old.IdIssuePublic, 40*24*time.Hour)

	first := loadIssuesPage(t, app, token, withinURL(idProject, "updateAtWithin=30d&limit=1"))
	require.Len(t, first.Items, 1)
	require.NotNil(t, first.NextCursor)

	// unchanged cursor → window still 30d from now → the 40-day-old row stays out
	fresh := loadIssuesPage(t, app, token,
		withinURL(idProject, "updateAtWithin=30d&limit=5&cursor="+*first.NextCursor))
	require.NotContains(t, publicIds(fresh), old.IdIssuePublic)

	// same cursor, reference moved 35 days back → window reaches 65 days → it appears
	shifted := reStampCursorRef(t, *first.NextCursor, time.Now().UTC().Add(-35*24*time.Hour))
	page := loadIssuesPage(t, app, token,
		withinURL(idProject, "updateAtWithin=30d&limit=5&cursor="+shifted))
	require.Contains(t, publicIds(page), old.IdIssuePublic,
		"the window must resolve against the cursor's reference, not a fresh now()")
}

// Rewrites only the pinned instant, leaving the keyset position intact.
func reStampCursorRef(t *testing.T, encoded string, ref time.Time) string {
	raw, err := base64.URLEncoding.DecodeString(encoded)
	require.Nil(t, err)
	var cursor map[string]any
	require.Nil(t, json.Unmarshal(raw, &cursor))
	cursor["n"] = ref.Format(time.RFC3339Nano)
	rewritten, err := json.Marshal(cursor)
	require.Nil(t, err)
	return base64.URLEncoding.EncodeToString(rewritten)
}

// Nothing to pin without a window, so the cursor stays exactly as it was.
func TestGetIssues_CursorCarriesNoRefWithoutWithin(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "within-cursor-absent")

	for i := 0; i < 3; i++ {
		createIssue(t, app, token, idProject, fmt.Sprintf("issue %d", i))
	}

	page := loadIssuesPage(t, app, token, withinURL(idProject, "limit=2"))
	require.NotNil(t, page.NextCursor)
	require.Empty(t, cursorRef(t, *page.NextCursor))
}

// Decoded by hand — the cursor type is deliberately private to the repository.
func cursorRef(t *testing.T, encoded string) string {
	raw, err := base64.URLEncoding.DecodeString(encoded)
	require.Nil(t, err)
	var cursor struct {
		WithinRef string `json:"n"`
	}
	require.Nil(t, json.Unmarshal(raw, &cursor))
	return cursor.WithinRef
}
