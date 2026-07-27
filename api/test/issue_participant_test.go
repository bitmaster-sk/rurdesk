package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

// participantUserIds extracts the idUser slice from a participant list.
// Used by later tasks in this file when asserting participant membership.
func participantUserIds(list []*model.IssueParticipant) []int64 {
	ids := make([]int64, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.IdUser)
	}
	return ids
}

// Test_Participant_CreateIssue_AutoAddsCreatorAndAssignee proves that after a
// successful CreateIssue HTTP call both the creator and the assignee (when set
// and different from the creator) appear as participants in the repository.
func Test_Participant_CreateIssue_AutoAddsCreatorAndAssignee(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	creatorToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"autocreator","email":"autocreator@test.sk","password":"kreslo"}`)
	creatorID := idOfUser(t, app, adminToken, "autocreator@test.sk")

	assigneeToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"autoassignee","email":"autoassignee@test.sk","password":"kreslo"}`)
	assigneeID := idOfUser(t, app, adminToken, "autoassignee@test.sk")

	idProject := createProject(t, app, creatorToken, "autopart-test-project")

	// Add assignee as project member so the ACL check inside CreateIssue passes.
	addRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, assigneeID),
		creatorToken)
	if addRes.StatusCode != http.StatusOK {
		t.Fatalf("add member: expected 200, got %d: %s", addRes.StatusCode, readBody(t, addRes))
	}
	_ = assigneeToken // created only to satisfy the helper; token not needed further

	issRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		fmt.Sprintf(`{"title":"Auto-part issue","description":"desc","assignedTo":%d}`, assigneeID),
		creatorToken)
	if issRes.StatusCode != http.StatusOK {
		t.Fatalf("create issue: expected 200, got %d: %s", issRes.StatusCode, readBody(t, issRes))
	}
	var iss model.Issue
	if err := json.NewDecoder(issRes.Body).Decode(&iss); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	idIssue := iss.IdIssue

	repo := repository.NewIssueParticipantRepository(app.Pool)

	list, err := repo.List(ctx, idIssue)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	ids := participantUserIds(list)
	if !containsInt64(ids, creatorID) {
		t.Errorf("creator %d not in participants %v", creatorID, ids)
	}
	if !containsInt64(ids, assigneeID) {
		t.Errorf("assignee %d not in participants %v", assigneeID, ids)
	}

	// Cleanup
	app.Pool.Exec(ctx, "DELETE FROM issues.issue_participant WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email IN ('autocreator@test.sk','autoassignee@test.sk')")
}

// containsInt64 reports whether needle is present in haystack.
func containsInt64(haystack []int64, needle int64) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

// Test_Participant_Add_Idempotent_DoesNotResetMute proves:
// 1. Add inserts a participant.
// 2. SetNotifications(false) mutes them.
// 3. A second Add (different source) does NOT reset has_notifications_enabled.
// 4. The muted user is absent from NotifiableUserIds.
func Test_Participant_Add_Idempotent_DoesNotResetMute(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	ownerToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"partowner","email":"partowner@test.sk","password":"kreslo"}`)
	ownerID := idOfUser(t, app, adminToken, "partowner@test.sk")

	idProject := createProject(t, app, ownerToken, "participant-idempotent-test")

	issRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"Part test","description":"desc"}`,
		ownerToken)
	if issRes.StatusCode != http.StatusOK {
		body := readBody(t, issRes)
		t.Fatalf("create issue: expected 200, got %d: %s", issRes.StatusCode, body)
	}
	var iss model.Issue
	if err := json.NewDecoder(issRes.Body).Decode(&iss); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	idIssue := iss.IdIssue

	repo := repository.NewIssueParticipantRepository(app.Pool)

	// First add — creator
	if err := repo.Add(ctx, idIssue, ownerID, "creator", &ownerID); err != nil {
		t.Fatalf("Add (creator): %v", err)
	}

	// Mute the user
	found, err := repo.SetNotifications(ctx, idIssue, ownerID, false)
	if err != nil {
		t.Fatalf("SetNotifications: %v", err)
	}
	if !found {
		t.Fatal("SetNotifications: expected found=true after Add")
	}

	// Re-add with different source — must be a no-op (ON CONFLICT DO NOTHING)
	if err := repo.Add(ctx, idIssue, ownerID, "comment", &ownerID); err != nil {
		t.Fatalf("Add (comment, re-add): %v", err)
	}

	// List must have exactly 1 row, still muted
	list, err := repo.List(ctx, idIssue)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(list))
	}
	if list[0].HasNotificationsEnabled {
		t.Error("re-add must NOT reset mute: HasNotificationsEnabled should still be false")
	}

	// Muted user must not appear in NotifiableUserIds
	notifiable, err := repo.NotifiableUserIds(ctx, idIssue)
	if err != nil {
		t.Fatalf("NotifiableUserIds: %v", err)
	}
	for _, idUser := range notifiable {
		if idUser == ownerID {
			t.Error("muted user must not be in NotifiableUserIds")
		}
	}

	// Cleanup
	app.Pool.Exec(ctx, "DELETE FROM issues.issue_participant WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email = 'partowner@test.sk'")
}

// Test_Participant_EditIssue_NewAssignee_BecomesParticipant proves that when an
// issue is edited via PATCH and a new assignee is set, that assignee is
// automatically added as a participant with source="assignee".
func Test_Participant_EditIssue_NewAssignee_BecomesParticipant(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	ownerToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"editpart_owner","email":"editpart_owner@test.sk","password":"kreslo"}`)
	ownerID := idOfUser(t, app, adminToken, "editpart_owner@test.sk")

	assigneeToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"editpart_assignee","email":"editpart_assignee@test.sk","password":"kreslo"}`)
	assigneeID := idOfUser(t, app, adminToken, "editpart_assignee@test.sk")
	_ = assigneeToken

	idProject := createProject(t, app, ownerToken, "editpart-test-project")

	// Add assignee as project member so ACL check passes.
	addRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, assigneeID),
		ownerToken)
	if addRes.StatusCode != http.StatusOK {
		t.Fatalf("add member: expected 200, got %d: %s", addRes.StatusCode, readBody(t, addRes))
	}

	// Create an issue without an assignee.
	issRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"Edit-part issue","description":"desc"}`,
		ownerToken)
	if issRes.StatusCode != http.StatusOK {
		t.Fatalf("create issue: expected 200, got %d: %s", issRes.StatusCode, readBody(t, issRes))
	}
	var iss model.Issue
	if err := json.NewDecoder(issRes.Body).Decode(&iss); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	idIssue := iss.IdIssue
	idIssuePublic := iss.IdIssuePublic

	// Edit the issue to assign the new assignee.
	editBody := fmt.Sprintf(
		`{"title":"Edit-part issue","description":"desc","assignedTo":%d}`,
		assigneeID,
	)
	editRes := Request(t, app, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d", idProject, idIssuePublic),
		editBody,
		ownerToken)
	if editRes.StatusCode != http.StatusOK {
		t.Fatalf("edit issue: expected 200, got %d: %s", editRes.StatusCode, readBody(t, editRes))
	}

	repo := repository.NewIssueParticipantRepository(app.Pool)

	// The new assignee must be a participant.
	list, err := repo.List(ctx, idIssue)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	ids := participantUserIds(list)
	if !containsInt64(ids, assigneeID) {
		t.Errorf("new assignee %d not in participants %v", assigneeID, ids)
	}

	// The creator (owner) must also be a participant (added by CreateIssue hook).
	if !containsInt64(ids, ownerID) {
		t.Errorf("creator/owner %d not in participants %v", ownerID, ids)
	}

	// Assert that the assignee's participant row has source="assignee".
	var assigneeSource string
	for _, p := range list {
		if p.IdUser == assigneeID {
			assigneeSource = p.Source
			break
		}
	}
	if assigneeSource != "assignee" {
		t.Errorf("expected source=assignee for new assignee, got %q", assigneeSource)
	}

	// The assignee must appear in NotifiableUserIds.
	notifiable, err := repo.NotifiableUserIds(ctx, idIssue)
	if err != nil {
		t.Fatalf("NotifiableUserIds: %v", err)
	}
	if !containsInt64(notifiable, assigneeID) {
		t.Errorf("new assignee %d not in notifiable participants %v", assigneeID, notifiable)
	}

	// Cleanup
	app.Pool.Exec(ctx, "DELETE FROM issues.issue_participant WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email IN ('editpart_owner@test.sk','editpart_assignee@test.sk')")
}

// Test_Participant_BulkEditIssues_NewAssignee_BecomesParticipant proves that
// when issues are batch-edited via PATCH /issue/batch and an assignee is set,
// that assignee is automatically added as a participant with source="assignee".
func Test_Participant_BulkEditIssues_NewAssignee_BecomesParticipant(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	ownerToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"bulkpart_owner","email":"bulkpart_owner@test.sk","password":"kreslo"}`)

	assigneeToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"bulkpart_assignee","email":"bulkpart_assignee@test.sk","password":"kreslo"}`)
	assigneeID := idOfUser(t, app, adminToken, "bulkpart_assignee@test.sk")
	_ = assigneeToken

	idProject := createProject(t, app, ownerToken, "bulkpart-test-project")

	// Add assignee as project member.
	addRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, assigneeID),
		ownerToken)
	if addRes.StatusCode != http.StatusOK {
		t.Fatalf("add member: expected 200, got %d: %s", addRes.StatusCode, readBody(t, addRes))
	}

	// Create an issue without an assignee.
	issRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"Bulk-part issue","description":"desc"}`,
		ownerToken)
	if issRes.StatusCode != http.StatusOK {
		t.Fatalf("create issue: expected 200, got %d: %s", issRes.StatusCode, readBody(t, issRes))
	}
	var iss model.Issue
	if err := json.NewDecoder(issRes.Body).Decode(&iss); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	idIssue := iss.IdIssue
	idIssuePublic := iss.IdIssuePublic

	// Bulk-edit: assign the new assignee via the batch endpoint.
	bulkBody := fmt.Sprintf(
		`{"issues":[{"idIssuePublic":%d,"idUserAssigned":%d}]}`,
		idIssuePublic, assigneeID,
	)
	bulkRes := Request(t, app, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/batch", idProject),
		bulkBody,
		ownerToken)
	if bulkRes.StatusCode != http.StatusOK {
		t.Fatalf("bulk edit: expected 200, got %d: %s", bulkRes.StatusCode, readBody(t, bulkRes))
	}

	repo := repository.NewIssueParticipantRepository(app.Pool)

	// The new assignee must be a participant.
	list, err := repo.List(ctx, idIssue)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	ids := participantUserIds(list)
	if !containsInt64(ids, assigneeID) {
		t.Errorf("bulk-assigned user %d not in participants %v", assigneeID, ids)
	}

	// Assert source="assignee" for the bulk-assigned participant row.
	var assigneeSource string
	for _, p := range list {
		if p.IdUser == assigneeID {
			assigneeSource = p.Source
			break
		}
	}
	if assigneeSource != "assignee" {
		t.Errorf("expected source=assignee for bulk-assigned user, got %q", assigneeSource)
	}

	// Cleanup
	app.Pool.Exec(ctx, "DELETE FROM issues.issue_participant WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email IN ('bulkpart_owner@test.sk','bulkpart_assignee@test.sk')")
}

// Test_Participant_HTTP_SetMyNotifications_SelfOnly proves that a project owner
// who created an issue can mute themselves via the PATCH endpoint, and that their
// id subsequently disappears from NotifiableUserIds.
func Test_Participant_HTTP_SetMyNotifications_SelfOnly(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	ownerToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"selfmute_owner","email":"selfmute_owner@test.sk","password":"kreslo"}`)
	ownerID := idOfUser(t, app, adminToken, "selfmute_owner@test.sk")

	idProject := createProject(t, app, ownerToken, "selfmute-test-project")

	// Create an issue — CreateIssue auto-adds the owner as a participant (source=creator).
	issRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"Selfmute issue","description":"desc"}`,
		ownerToken)
	if issRes.StatusCode != http.StatusOK {
		t.Fatalf("create issue: expected 200, got %d: %s", issRes.StatusCode, readBody(t, issRes))
	}
	var iss model.Issue
	if err := json.NewDecoder(issRes.Body).Decode(&iss); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	idIssue := iss.IdIssue
	idIssuePublic := iss.IdIssuePublic

	// Mute self via PATCH endpoint.
	muteRes := Request(t, app, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d/participant/notifications", idProject, idIssuePublic),
		`{"enabled":false}`,
		ownerToken)
	if muteRes.StatusCode != http.StatusOK {
		t.Fatalf("set notifications: expected 200, got %d: %s", muteRes.StatusCode, readBody(t, muteRes))
	}

	// Owner must no longer appear in NotifiableUserIds.
	repo := repository.NewIssueParticipantRepository(app.Pool)
	notifiable, err := repo.NotifiableUserIds(ctx, idIssue)
	if err != nil {
		t.Fatalf("NotifiableUserIds: %v", err)
	}
	if containsInt64(notifiable, ownerID) {
		t.Errorf("muted owner %d must not appear in NotifiableUserIds %v", ownerID, notifiable)
	}

	// Cleanup
	app.Pool.Exec(ctx, "DELETE FROM issues.issue_participant WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email = 'selfmute_owner@test.sk'")
}

// Test_Participant_HTTP_SetMyNotifications_NotParticipant proves that a user who
// is a project member but not yet a participant gets 404 when calling the mute
// endpoint.
func Test_Participant_HTTP_SetMyNotifications_NotParticipant(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	ownerToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"notpart_owner","email":"notpart_owner@test.sk","password":"kreslo"}`)

	memberToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"notpart_member","email":"notpart_member@test.sk","password":"kreslo"}`)
	memberID := idOfUser(t, app, adminToken, "notpart_member@test.sk")

	idProject := createProject(t, app, ownerToken, "notpart-test-project")

	// Add member to project.
	addRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, memberID),
		ownerToken)
	if addRes.StatusCode != http.StatusOK {
		t.Fatalf("add member: expected 200, got %d: %s", addRes.StatusCode, readBody(t, addRes))
	}

	// Create issue — member is NOT the creator and not auto-added.
	issRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"Notpart issue","description":"desc"}`,
		ownerToken)
	if issRes.StatusCode != http.StatusOK {
		t.Fatalf("create issue: expected 200, got %d: %s", issRes.StatusCode, readBody(t, issRes))
	}
	var iss model.Issue
	if err := json.NewDecoder(issRes.Body).Decode(&iss); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	idIssuePublic := iss.IdIssuePublic

	// Member tries to mute themselves — they are not a participant → expect 404.
	muteRes := Request(t, app, "PATCH",
		fmt.Sprintf("/api/private/project/%d/issue/%d/participant/notifications", idProject, idIssuePublic),
		`{"enabled":false}`,
		memberToken)
	if muteRes.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for non-participant self-mute, got %d", muteRes.StatusCode)
	}

	// Cleanup
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = $1", iss.IdIssue)
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email IN ('notpart_owner@test.sk','notpart_member@test.sk')")
}

// Test_Participant_HTTP_AddParticipant_OutsiderForbidden proves that attempting
// to manually add a user who is not a project member returns 403.
func Test_Participant_HTTP_AddParticipant_OutsiderForbidden(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	ownerToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"addpart_owner","email":"addpart_owner@test.sk","password":"kreslo"}`)

	// Outsider: registered user but NOT added to the project.
	_ = createUserAsAdmin(t, app, adminToken,
		`{"name":"addpart_outsider","email":"addpart_outsider@test.sk","password":"kreslo"}`)
	outsiderID := idOfUser(t, app, adminToken, "addpart_outsider@test.sk")

	idProject := createProject(t, app, ownerToken, "addpart-test-project")

	issRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"Addpart issue","description":"desc"}`,
		ownerToken)
	if issRes.StatusCode != http.StatusOK {
		t.Fatalf("create issue: expected 200, got %d: %s", issRes.StatusCode, readBody(t, issRes))
	}
	var iss model.Issue
	if err := json.NewDecoder(issRes.Body).Decode(&iss); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	idIssuePublic := iss.IdIssuePublic

	// Attempt to add the outsider — must be rejected with 403.
	addRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue/%d/participant", idProject, idIssuePublic),
		fmt.Sprintf(`{"idUser":%d}`, outsiderID),
		ownerToken)
	if addRes.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 adding outsider, got %d", addRes.StatusCode)
	}

	// Cleanup
	app.Pool.Exec(ctx, "DELETE FROM issues.issue_participant WHERE id_issue = $1", iss.IdIssue)
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = $1", iss.IdIssue)
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email IN ('addpart_owner@test.sk','addpart_outsider@test.sk')")
}

// Test_Participant_HTTP_AddParticipant_MemberSucceeds proves that a project
// member can be added as a participant via the POST endpoint and subsequently
// appears in the participant list.
func Test_Participant_HTTP_AddParticipant_MemberSucceeds(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	ownerToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"addmember_owner","email":"addmember_owner@test.sk","password":"kreslo"}`)

	_ = createUserAsAdmin(t, app, adminToken,
		`{"name":"addmember_member","email":"addmember_member@test.sk","password":"kreslo"}`)
	memberID := idOfUser(t, app, adminToken, "addmember_member@test.sk")

	idProject := createProject(t, app, ownerToken, "addmember-test-project")

	// Add member to project.
	addMemberRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, memberID),
		ownerToken)
	if addMemberRes.StatusCode != http.StatusOK {
		t.Fatalf("add project member: expected 200, got %d: %s", addMemberRes.StatusCode, readBody(t, addMemberRes))
	}

	issRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"Addmember issue","description":"desc"}`,
		ownerToken)
	if issRes.StatusCode != http.StatusOK {
		t.Fatalf("create issue: expected 200, got %d: %s", issRes.StatusCode, readBody(t, issRes))
	}
	var iss model.Issue
	if err := json.NewDecoder(issRes.Body).Decode(&iss); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	idIssue := iss.IdIssue
	idIssuePublic := iss.IdIssuePublic

	// Add the project member as a participant.
	addRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue/%d/participant", idProject, idIssuePublic),
		fmt.Sprintf(`{"idUser":%d}`, memberID),
		ownerToken)
	if addRes.StatusCode != http.StatusOK {
		t.Fatalf("add participant: expected 200, got %d: %s", addRes.StatusCode, readBody(t, addRes))
	}

	// Verify the member now appears in the participant list.
	repo := repository.NewIssueParticipantRepository(app.Pool)
	list, err := repo.List(ctx, idIssue)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	ids := participantUserIds(list)
	if !containsInt64(ids, memberID) {
		t.Errorf("added member %d not in participants %v", memberID, ids)
	}

	// Cleanup
	app.Pool.Exec(ctx, "DELETE FROM issues.issue_participant WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email IN ('addmember_owner@test.sk','addmember_member@test.sk')")
}
