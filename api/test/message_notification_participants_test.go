package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/repository"
)

// notificationsForUser fetches the persistent notification list for the
// authenticated user via GET /api/private/notification.
func notificationsForUser(t *testing.T, app *issue.Application, token string) []*model.Notification {
	t.Helper()
	res := Request(t, app, "GET", "/api/private/notification", "", token)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/private/notification: expected 200, got %d: %s", res.StatusCode, readBody(t, res))
	}
	var notifications []*model.Notification
	if err := json.NewDecoder(res.Body).Decode(&notifications); err != nil {
		t.Fatalf("decode notifications: %v", err)
	}
	return notifications
}

// Test_Message_IssueComment_NotifiesOnlyParticipants_NotAllMembers verifies
// that when a project member posts a comment on an issue, the persistent
// notification (notifSvc.Notify) goes ONLY to participants, not to every
// project member. A bystander who is merely a project member but NOT a
// participant must receive no notification.
func Test_Message_IssueComment_NotifiesOnlyParticipants_NotAllMembers(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	ownerToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"notifpart_owner","email":"notifpart_owner@test.sk","password":"kreslo"}`)
	ownerID := idOfUser(t, app, adminToken, "notifpart_owner@test.sk")

	participantToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"notifpart_participant","email":"notifpart_participant@test.sk","password":"kreslo"}`)
	participantID := idOfUser(t, app, adminToken, "notifpart_participant@test.sk")

	bystanderToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"notifpart_bystander","email":"notifpart_bystander@test.sk","password":"kreslo"}`)
	bystanderID := idOfUser(t, app, adminToken, "notifpart_bystander@test.sk")

	idProject := createProject(t, app, ownerToken, "notifpart-test-project")

	// Add participant and bystander as project members.
	for _, memberID := range []int64{participantID, bystanderID} {
		addRes := Request(t, app, "POST",
			fmt.Sprintf("/api/private/project/%d/member/user", idProject),
			fmt.Sprintf(`{"idUser":%d,"role":"member"}`, memberID),
			ownerToken)
		if addRes.StatusCode != http.StatusOK {
			t.Fatalf("add member %d: expected 200, got %d: %s", memberID, addRes.StatusCode, readBody(t, addRes))
		}
	}

	// Owner creates an issue and assigns participant — both become participants
	// (creator=owner via source="creator", assignee=participant via source="assignee").
	issRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		fmt.Sprintf(`{"title":"Notif test issue","description":"desc","assignedTo":%d}`, participantID),
		ownerToken)
	if issRes.StatusCode != http.StatusOK {
		t.Fatalf("create issue: expected 200, got %d: %s", issRes.StatusCode, readBody(t, issRes))
	}
	var iss model.Issue
	if err := json.NewDecoder(issRes.Body).Decode(&iss); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	idIssue := iss.IdIssue

	// Verify: at this point bystander is NOT a participant.
	partRepo := repository.NewIssueParticipantRepository(app.Pool)
	partList, err := partRepo.List(ctx, idIssue)
	if err != nil {
		t.Fatalf("List participants: %v", err)
	}
	partIds := participantUserIds(partList)
	if containsInt64(partIds, bystanderID) {
		t.Fatalf("bystander %d should not be a participant before the comment", bystanderID)
	}

	// Participant posts a comment on the issue (becomes author → participant with source="comment").
	commentBody := fmt.Sprintf(
		`{"idMessageRecipientType":4,"idRecipient":%d,"message":"hey owner check this out"}`,
		idIssue,
	)
	commentRes := Request(t, app, "POST", "/api/private/message", commentBody, participantToken)
	if commentRes.StatusCode != http.StatusOK {
		t.Fatalf("post comment: expected 200, got %d: %s", commentRes.StatusCode, readBody(t, commentRes))
	}

	issueRefId := fmt.Sprintf("%d", idIssue)

	// Owner (a participant) should have received a notification.
	// Note: Notification.IdUser has json:"-" so the API response never exposes it;
	// use a direct DB count to assert the owner received a notification.
	var ownerNotifCount int
	if err := app.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM notification.notification WHERE id_user=$1 AND ref_id=$2",
		ownerID, issueRefId,
	).Scan(&ownerNotifCount); err != nil {
		t.Fatalf("querying owner notifications: %v", err)
	}
	if ownerNotifCount == 0 {
		t.Errorf("owner (participant) expected to have a notification for issue %d, got none", idIssue)
	}

	// Bystander (NOT a participant) must NOT have received any notification for
	// this issue.
	var bystanderNotifCount int
	if err := app.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM notification.notification WHERE id_user=$1 AND ref_id=$2",
		bystanderID, issueRefId,
	).Scan(&bystanderNotifCount); err != nil {
		t.Fatalf("querying bystander notifications: %v", err)
	}
	if bystanderNotifCount > 0 {
		t.Errorf("bystander (not a participant) must NOT receive a notification for issue %d, got %d", idIssue, bystanderNotifCount)
	}

	// API-level check: owner's notification endpoint returns at least one entry for this issue.
	ownerNotifs := notificationsForUser(t, app, ownerToken)
	foundForOwner := false
	for _, n := range ownerNotifs {
		if n.RefId == issueRefId {
			foundForOwner = true
			break
		}
	}
	if !foundForOwner {
		t.Errorf("owner notification API: expected at least one notification with refId=%s", issueRefId)
	}

	// Bystander API check: none of their notifications reference this issue.
	bystanderNotifs := notificationsForUser(t, app, bystanderToken)
	for _, n := range bystanderNotifs {
		if n.RefId == issueRefId {
			t.Errorf("bystander notification API: found unexpected notification with refId=%s", issueRefId)
		}
	}

	// Cleanup
	app.Pool.Exec(ctx, "DELETE FROM notification.notification WHERE ref_id = $1", fmt.Sprintf("%d", idIssue))
	app.Pool.Exec(ctx, "DELETE FROM issues.issue_participant WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email IN ('notifpart_owner@test.sk','notifpart_participant@test.sk','notifpart_bystander@test.sk')")
}

// Test_Message_IssueComment_MentionAddsParticipantAndNotifies verifies that
// when a comment @mentions a project member who was not previously a
// participant, that member:
//  1. becomes a notifiable participant (source="mention"), and
//  2. receives a persistent notification.
func Test_Message_IssueComment_MentionAddsParticipantAndNotifies(t *testing.T) {
	app := Setup(t)
	ctx := context.Background()
	adminToken := Token(t, app)

	ownerToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"mentionpart_owner","email":"mentionpart_owner@test.sk","password":"kreslo"}`)

	// mentioned user has a name unique enough to avoid substring false-positives
	// against other seeded users ("Xanthe" does not appear anywhere else).
	mentionedToken := createUserAsAdmin(t, app, adminToken,
		`{"name":"Xanthe","email":"mentionpart_mentioned@test.sk","password":"kreslo"}`)
	mentionedID := idOfUser(t, app, adminToken, "mentionpart_mentioned@test.sk")

	idProject := createProject(t, app, ownerToken, "mentionpart-test-project")

	// Add the mentioned user as a project member (required for ACL + mention detection).
	addRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/member/user", idProject),
		fmt.Sprintf(`{"idUser":%d,"role":"member"}`, mentionedID),
		ownerToken)
	if addRes.StatusCode != http.StatusOK {
		t.Fatalf("add member: expected 200, got %d: %s", addRes.StatusCode, readBody(t, addRes))
	}

	// Owner creates an issue (no assignee → only owner is participant initially).
	issRes := Request(t, app, "POST",
		fmt.Sprintf("/api/private/project/%d/issue", idProject),
		`{"title":"Mention test issue","description":"desc"}`,
		ownerToken)
	if issRes.StatusCode != http.StatusOK {
		t.Fatalf("create issue: expected 200, got %d: %s", issRes.StatusCode, readBody(t, issRes))
	}
	var iss model.Issue
	if err := json.NewDecoder(issRes.Body).Decode(&iss); err != nil {
		t.Fatalf("decode issue: %v", err)
	}
	idIssue := iss.IdIssue

	// Owner posts a comment that @mentions Xanthe using the token format @[name](user:<id>).
	commentBody := fmt.Sprintf(
		`{"idMessageRecipientType":4,"idRecipient":%d,"message":"cc @[Xanthe](user:%d) please review"}`,
		idIssue, mentionedID,
	)
	commentRes := Request(t, app, "POST", "/api/private/message", commentBody, ownerToken)
	if commentRes.StatusCode != http.StatusOK {
		t.Fatalf("post comment: expected 200, got %d: %s", commentRes.StatusCode, readBody(t, commentRes))
	}

	// Xanthe must now be a notifiable participant.
	partRepo := repository.NewIssueParticipantRepository(app.Pool)
	notifiable, err := partRepo.NotifiableUserIds(ctx, idIssue)
	if err != nil {
		t.Fatalf("NotifiableUserIds: %v", err)
	}
	if !containsInt64(notifiable, mentionedID) {
		t.Errorf("mentioned user %d must be a notifiable participant; notifiable list: %v", mentionedID, notifiable)
	}

	// Xanthe must have received a notification for this issue.
	// Note: Notification.IdUser has json:"-"; use a direct DB count.
	issueRefId := fmt.Sprintf("%d", idIssue)
	var mentionedNotifCount int
	if err := app.Pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM notification.notification WHERE id_user=$1 AND ref_id=$2",
		mentionedID, issueRefId,
	).Scan(&mentionedNotifCount); err != nil {
		t.Fatalf("querying mentioned notifications: %v", err)
	}
	if mentionedNotifCount == 0 {
		t.Errorf("mentioned user %d expected to have a notification for issue %d, got none", mentionedID, idIssue)
	}

	// The notification for the @mentioned user must be of type "mention", not "comment".
	// The body must contain "@Xanthe" (display name) and NOT the raw "user:" token.
	var mentionedNotifType, mentionedNotifBody string
	if err := app.Pool.QueryRow(ctx,
		"SELECT type, body FROM notification.notification WHERE id_user=$1 AND ref_id=$2 LIMIT 1",
		mentionedID, issueRefId,
	).Scan(&mentionedNotifType, &mentionedNotifBody); err != nil {
		t.Fatalf("querying mentioned notification type/body: %v", err)
	}
	if mentionedNotifType != "mention" {
		t.Errorf("mentioned user notification type: expected %q, got %q", "mention", mentionedNotifType)
	}
	if !strings.Contains(mentionedNotifBody, "@Xanthe") {
		t.Errorf("notification body should contain @Xanthe, got: %q", mentionedNotifBody)
	}
	if strings.Contains(mentionedNotifBody, "user:") {
		t.Errorf("notification body must not contain raw token 'user:', got: %q", mentionedNotifBody)
	}

	// Also verify via API that Xanthe has at least one notification for this issue.
	mentionedNotifs := notificationsForUser(t, app, mentionedToken)
	foundForMentioned := false
	for _, n := range mentionedNotifs {
		if n.RefId == issueRefId {
			foundForMentioned = true
			break
		}
	}
	if !foundForMentioned {
		t.Errorf("mentioned user %d notification API: expected notification for issue %d, got none", mentionedID, idIssue)
	}

	// Mentioning a non-member ID must NOT create a participant entry.
	// Use a large ID unlikely to collide with any seeded user.
	const nonMemberID int64 = 999999999
	nonMemberCommentBody := fmt.Sprintf(
		`{"idMessageRecipientType":4,"idRecipient":%d,"message":"hey @[Ghost](user:%d) you there?"}`,
		idIssue, nonMemberID,
	)
	nonMemberCommentRes := Request(t, app, "POST", "/api/private/message", nonMemberCommentBody, ownerToken)
	if nonMemberCommentRes.StatusCode != http.StatusOK {
		t.Fatalf("post non-member mention comment: expected 200, got %d: %s", nonMemberCommentRes.StatusCode, readBody(t, nonMemberCommentRes))
	}
	notifiableAfterNonMember, err := partRepo.NotifiableUserIds(ctx, idIssue)
	if err != nil {
		t.Fatalf("NotifiableUserIds after non-member mention: %v", err)
	}
	if containsInt64(notifiableAfterNonMember, nonMemberID) {
		t.Errorf("non-member user %d must NOT become a participant after being mentioned", nonMemberID)
	}

	// Cleanup
	app.Pool.Exec(ctx, "DELETE FROM notification.notification WHERE ref_id = $1", fmt.Sprintf("%d", idIssue))
	app.Pool.Exec(ctx, "DELETE FROM issues.issue_participant WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM issues.issue WHERE id_issue = $1", idIssue)
	app.Pool.Exec(ctx, "DELETE FROM projects.project WHERE id_project = $1", idProject)
	app.Pool.Exec(ctx, "DELETE FROM users.user WHERE email IN ('mentionpart_owner@test.sk','mentionpart_mentioned@test.sk')")
}
