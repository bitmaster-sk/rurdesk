package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/errs"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/bitmaster-sk/rurdesk/api/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMessageAcl struct {
	readableTeams    map[int64]bool
	readableProjects map[int64]bool
}

func (f *fakeMessageAcl) CanReadTeam(_ context.Context, _, idTeam int64) bool {
	return f.readableTeams[idTeam]
}

func (f *fakeMessageAcl) CanReadProject(_ context.Context, _, idProject int64) bool {
	return f.readableProjects[idProject]
}

type fakeIssueProjectLoader struct {
	projectByIssue map[int64]int64
}

var errIssueNotFound = errors.New("issue not found")

func (f *fakeIssueProjectLoader) LoadProjectByIssue(_ context.Context, idIssue int64) (*model.Project, error) {
	idProject, ok := f.projectByIssue[idIssue]
	if !ok {
		return nil, errIssueNotFound
	}
	return &model.Project{IdProject: idProject}, nil
}

type fakeUserLoader struct {
	existingUsers map[int64]bool
}

func (f *fakeUserLoader) LoadUser(_ context.Context, idUser int64) (*model.User, error) {
	if !f.existingUsers[idUser] {
		return nil, errors.New("user not found")
	}
	return &model.User{IdUser: idUser}, nil
}

func TestMessageAccessService_CheckContextAccess(t *testing.T) {
	acl := &fakeMessageAcl{
		readableTeams:    map[int64]bool{10: true},
		readableProjects: map[int64]bool{20: true},
	}
	projects := &fakeIssueProjectLoader{projectByIssue: map[int64]int64{100: 20, 200: 21}}
	users := &fakeUserLoader{existingUsers: map[int64]bool{5: true}}
	svc := service.NewMessageAccessService(acl, projects, users)

	cases := []struct {
		name          string
		recipientType model.MessageRecipientType
		idRecipient   int64
		wantErr       error
	}{
		{"direct message to existing user", model.TeammateRecipientType, 5, nil},
		{"direct message to missing user", model.TeammateRecipientType, 6, errs.ErrForbidden},
		{"member team", model.TeamRecipientType, 10, nil},
		{"foreign team", model.TeamRecipientType, 11, errs.ErrForbidden},
		{"readable project", model.ProjectRecipientType, 20, nil},
		{"foreign project", model.ProjectRecipientType, 21, errs.ErrForbidden},
		{"issue in readable project", model.IssueRecipientType, 100, nil},
		{"issue in foreign project", model.IssueRecipientType, 200, errs.ErrForbidden},
		{"unknown issue", model.IssueRecipientType, 300, errIssueNotFound},
		{"unknown recipient type", model.MessageRecipientType(99), 1, errs.ErrBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.CheckContextAccess(context.Background(), 1, tc.recipientType, tc.idRecipient)
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}
