package test

import (
	"context"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/agent"
	"github.com/bitmaster-sk/rurdesk/api/internal/constants"
	"github.com/bitmaster-sk/rurdesk/api/internal/controller"
	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/notify"
	"github.com/stretchr/testify/require"
)

// A notifier without the listen loop, so the test drains what the broadcast sent.
func captureNotifier() *notify.Notifier {
	return &notify.Notifier{Send: make(chan *notify.Notice, 32)}
}

func broadcastController(notifier *notify.Notifier) *controller.CustomFieldController {
	return controller.NewCustomFieldController(nil, nil, nil, notifier, injector.GetProjectRepository())
}

func drainNotice(t *testing.T, notifier *notify.Notifier) *notify.Notice {
	t.Helper()
	select {
	case notice := <-notifier.Send:
		return notice
	case <-time.After(2 * time.Second):
		t.Fatal("no notice was broadcast")
		return nil
	}
}

func TestIssueUpdateNotice_CarriesCustomFields(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-notice-issue")

	createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText})
	created := createIssueWithCustomFields(t, app, token, idProject, "noticed", map[string]any{"note": "hello"})

	notifier := captureNotifier()
	agent.BroadcastIssueUpdate(context.Background(), notifier,
		injector.GetIssueRepository(), injector.GetProjectRepository(), created.IdIssue)

	notice := drainNotice(t, notifier)
	require.Equal(t, notify.SubjectIssue, notice.Subject)

	issue, ok := notice.Payload.(*model.Issue)
	require.True(t, ok, "issue notice must carry *model.Issue")
	require.Equal(t, "hello", issue.CustomFields["note"])
}

func TestCustomFieldNotice_ReachesProjectMembersOnly(t *testing.T) {
	app := Setup(t)
	ownerToken := Token(t, app)
	idProject := createProject(t, app, ownerToken, "cf-notice-members")
	field := createCustomField(t, app, ownerToken, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "note", Name: "Note", FieldType: constants.CustomFieldTypeText})

	createUserAndLogin(t, app, ownerToken, "cf-notice-stranger@test.sk")
	var idStranger int64
	require.Nil(t, app.Pool.QueryRow(context.Background(),
		`SELECT id_user FROM users.user WHERE email = 'cf-notice-stranger@test.sk'`).Scan(&idStranger))

	notifier := captureNotifier()
	broadcastController(notifier).BroadcastCustomField(context.Background(), &field, notify.ActionCreate)

	notice := drainNotice(t, notifier)
	require.Equal(t, notify.SubjectCustomField, notice.Subject)
	require.NotEmpty(t, notice.IdsUser)
	require.NotContains(t, notice.IdsUser, idStranger)
}

func TestCustomFieldDeleteNotice_CarriesTheDeletedDefinition(t *testing.T) {
	app := Setup(t)
	token := Token(t, app)
	idProject := createProject(t, app, token, "cf-notice-delete")
	field := createCustomField(t, app, token, model.CreateCustomFieldReq{
		IdProject: idProject, Key: "gone", Name: "Gone", FieldType: constants.CustomFieldTypeText})

	notifier := captureNotifier()
	broadcastController(notifier).BroadcastCustomField(context.Background(), &field, notify.ActionDelete)

	notice := drainNotice(t, notifier)
	require.Equal(t, notify.ActionDelete, notice.Action)

	payload, ok := notice.Payload.(*model.CustomField)
	require.True(t, ok, "delete notice must carry the definition the client drops values by")
	require.Equal(t, "gone", payload.Key)
}
