package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/require"
)

// GetRelationsBulk's idsIssue param moved from a local comma-only split to the
// shared urlutil.ParseInt64Array, so the API has one array-param code path.
// Two pre-existing quirks shape the assertions:
//   - idsIssue filters on the INTERNAL id_issue (issue_relation_repository.go:70),
//     not idIssuePublic — hence IdIssue.
//   - UNION ALL over an outbound + inbound half, so one relation can yield two
//     rows. Assert on distinct relation ids, not row counts.

func TestGetRelationsBulk_AcceptsBothIdFormsAfterHelperMigration(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "relation-ids")

	from := createIssue(t, app, token, idProject, "relation from")
	to := createIssue(t, app, token, idProject, "relation to")
	unrelated := createIssue(t, app, token, idProject, "relation unrelated")
	other := createIssue(t, app, token, idProject, "relation other")

	createRelation(t, app, token, idProject, from.IdIssuePublic, to.IdIssuePublic)
	createRelation(t, app, token, idProject, unrelated.IdIssuePublic, other.IdIssuePublic)

	base := fmt.Sprintf("/api/private/project/%d/relation", idProject)

	// unfiltered: both relations are visible
	all := relationIdSet(loadRelations(t, app, token, base))
	require.Len(t, all, 2, "two relations exist in the project")

	// comma-joined — the form this endpoint accepted before the migration
	comma := relationIdSet(loadRelations(t, app, token,
		fmt.Sprintf("%s?idsIssue=%d", base, from.IdIssue)))
	require.Len(t, comma, 1,
		"comma-joined idsIssue must still filter after moving to the shared helper")

	// several comma-joined ids — the case that was broken for the issue filter
	commaMulti := relationIdSet(loadRelations(t, app, token,
		fmt.Sprintf("%s?idsIssue=%d,%d", base, from.IdIssue, to.IdIssue)))
	require.Equal(t, comma, commaMulti,
		"adding the other end of the same relation must not widen the result")

	// repeated — newly accepted, so any client may use either form
	repeated := relationIdSet(loadRelations(t, app, token,
		fmt.Sprintf("%s?idsIssue=%d&idsIssue=%d", base, from.IdIssue, to.IdIssue)))
	require.Equal(t, commaMulti, repeated, "both wire forms must select the same relations")
}

// Deliberate contract change, asserted so it is not a surprise: this endpoint
// answered 400 when an element did not parse. It now skips it, matching the four
// issue-filter params — a filter degrades rather than rejecting the request.
func TestGetRelationsBulk_InvalidIdElementIsSkippedNot400(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "relation-ids-invalid")

	from := createIssue(t, app, token, idProject, "relation from")
	to := createIssue(t, app, token, idProject, "relation to")
	createRelation(t, app, token, idProject, from.IdIssuePublic, to.IdIssuePublic)

	url := fmt.Sprintf("/api/private/project/%d/relation?idsIssue=%d,abc", idProject, from.IdIssue)
	res := Request(t, app, "GET", url, "", token)
	require.Equal(t, http.StatusOK, res.StatusCode,
		"was 400 before the helper migration; now the bad element is skipped")

	var relations []model.ReadIssueRelationRes
	require.Nil(t, json.NewDecoder(res.Body).Decode(&relations))
	require.Len(t, relations, 1)
}

// Nothing parseable means no filter, not an empty result. Asserted against the
// unfiltered response so it cannot drift with the UNION ALL row multiplicity.
func TestGetRelationsBulk_AllInvalidIdsMeansNoFilter(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "relation-ids-all-invalid")

	from := createIssue(t, app, token, idProject, "relation from")
	to := createIssue(t, app, token, idProject, "relation to")
	unrelated := createIssue(t, app, token, idProject, "relation unrelated")
	other := createIssue(t, app, token, idProject, "relation other")
	createRelation(t, app, token, idProject, from.IdIssuePublic, to.IdIssuePublic)
	createRelation(t, app, token, idProject, unrelated.IdIssuePublic, other.IdIssuePublic)

	base := fmt.Sprintf("/api/private/project/%d/relation", idProject)
	unfiltered := relationIdSet(loadRelations(t, app, token, base))
	allInvalid := relationIdSet(loadRelations(t, app, token, base+"?idsIssue=abc,def"))

	require.Len(t, unfiltered, 2)
	require.Equal(t, unfiltered, allInvalid,
		"no parseable id means no filter, not an empty result")
}

// createRelation lives in issue_paging_test.go (same package) — reused here.

func createIssue(t *testing.T, app *issue.Application, token string, idProject int64, title string) model.Issue {
	body, _ := json.Marshal(model.CreateIssueReq{
		IdProject:   idProject,
		Title:       title,
		Description: "x",
	})
	res := Request(t, app, "POST", "/api/private/project/"+itoa(idProject)+"/issue", string(body), token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var created model.Issue
	require.Nil(t, json.NewDecoder(res.Body).Decode(&created))
	return created
}

// relationIdSet collapses the outbound/inbound duplicates the UNION ALL query
// produces, so assertions are about WHICH relations matched, not how many rows.
func relationIdSet(relations []model.ReadIssueRelationRes) map[int64]bool {
	set := make(map[int64]bool, len(relations))
	for _, relation := range relations {
		set[relation.IdIssueRelation] = true
	}
	return set
}

func loadRelations(t *testing.T, app *issue.Application, token, url string) []model.ReadIssueRelationRes {
	res := Request(t, app, "GET", url, "", token)
	require.Equal(t, http.StatusOK, res.StatusCode)
	var relations []model.ReadIssueRelationRes
	require.Nil(t, json.NewDecoder(res.Body).Decode(&relations))
	return relations
}
