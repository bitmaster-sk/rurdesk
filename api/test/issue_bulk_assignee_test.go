package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
)

func Test_BulkEditIssues_AssigneeNullClears_AbsentKeeps(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	ownerToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"bulkassign_owner","email":"bulkassign_owner@test.sk","password":"kreslo"}`)
	ownerID := idOfUser(t, app, adminToken, "bulkassign_owner@test.sk")
	idProject := createProject(t, app, ownerToken, "bulkassign-test-project")

	createAssigned := func(title string) model.Issue {
		res := Request(t, app, "POST",
			fmt.Sprintf("/api/private/project/%d/issue", idProject),
			fmt.Sprintf(`{"title":%q,"description":"desc","assignedTo":%d}`, title, ownerID),
			ownerToken)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("create issue: expected 200, got %d: %s", res.StatusCode, readBody(t, res))
		}
		var iss model.Issue
		if err := json.NewDecoder(res.Body).Decode(&iss); err != nil {
			t.Fatalf("decode issue: %v", err)
		}
		return iss
	}
	cleared := createAssigned("Bulk unassign")
	kept := createAssigned("Bulk keep")

	res := Request(t, app, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/batch", idProject),
		fmt.Sprintf(`{"issues":[{"idIssuePublic":%d,"idUserAssigned":null},{"idIssuePublic":%d}]}`,
			cleared.IdIssuePublic, kept.IdIssuePublic),
		ownerToken)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("bulk edit: expected 200, got %d: %s", res.StatusCode, readBody(t, res))
	}
	var updated []model.Issue
	if err := json.NewDecoder(res.Body).Decode(&updated); err != nil {
		t.Fatalf("decode bulk response: %v", err)
	}

	byPublic := make(map[int64]model.Issue, len(updated))
	for _, iss := range updated {
		byPublic[iss.IdIssuePublic] = iss
	}
	if got := byPublic[cleared.IdIssuePublic].AssignedTo; got != nil {
		t.Errorf("explicit null must clear the assignee, got %d", *got)
	}
	if got := byPublic[kept.IdIssuePublic].AssignedTo; got == nil || *got != ownerID {
		t.Errorf("absent idUserAssigned must keep the assignee %d, got %v", ownerID, got)
	}

	app.Pool.Exec(ctx, "DELETE FROM issues.issue_participant WHERE id_issue = ANY($1)", []int64{cleared.IdIssue, kept.IdIssue})
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = ANY($1)", []int64{cleared.IdIssue, kept.IdIssue})
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email = 'bulkassign_owner@test.sk'")
}

func Test_BulkEditIssues_NotifiesNewAssigneeOncePerTask(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	ownerToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"bulknotify_owner","email":"bulknotify_owner@test.sk","password":"kreslo"}`)
	createUserAsAdmin(t, app, adminToken,
		`{"name":"bulknotify_member","email":"bulknotify_member@test.sk","password":"kreslo"}`)
	memberID := idOfUser(t, app, adminToken, "bulknotify_member@test.sk")
	idProject := createProject(t, app, ownerToken, "bulknotify-test-project")

	res := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, memberID),
		ownerToken)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("add member: expected 200, got %d: %s", res.StatusCode, readBody(t, res))
	}

	create := func(title string) model.Issue {
		res := Request(t, app, "POST",
			fmt.Sprintf("/api/private/project/%d/issue", idProject),
			fmt.Sprintf(`{"title":%q,"description":"desc"}`, title),
			ownerToken)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("create issue: expected 200, got %d: %s", res.StatusCode, readBody(t, res))
		}
		var iss model.Issue
		if err := json.NewDecoder(res.Body).Decode(&iss); err != nil {
			t.Fatalf("decode issue: %v", err)
		}
		return iss
	}
	first := create("Bulk notify one")
	second := create("Bulk notify two")

	res = Request(t, app, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/batch", idProject),
		fmt.Sprintf(`{"issues":[{"idIssuePublic":%d,"idUserAssigned":%d},{"idIssuePublic":%d,"idUserAssigned":%d}]}`,
			first.IdIssuePublic, memberID, second.IdIssuePublic, memberID),
		ownerToken)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("bulk edit: expected 200, got %d: %s", res.StatusCode, readBody(t, res))
	}

	var count int
	if err := app.Pool.QueryRow(ctx,
		`SELECT count(*) FROM notification.notification
		 WHERE id_user = $1 AND type = 'assigned' AND id_project = $2`,
		memberID, idProject).Scan(&count); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 2 {
		t.Errorf("expected one assigned notification per task (2), got %d", count)
	}

	app.Pool.Exec(ctx, "DELETE FROM notification.notification WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM issues.issue_participant WHERE id_issue = ANY($1)", []int64{first.IdIssue, second.IdIssue})
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = ANY($1)", []int64{first.IdIssue, second.IdIssue})
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email IN ('bulknotify_owner@test.sk', 'bulknotify_member@test.sk')")
}
