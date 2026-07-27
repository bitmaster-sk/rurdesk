package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/model"
	"github.com/stretchr/testify/suite"
)

// MessageDmSuite verifies that any user can DM any other user
// (no shared team required) and that /teammates is gone.
type MessageDmSuite struct {
	suite.Suite
	App    *issue.Application
	TokenA string
	IdB    int64
}

func (s *MessageDmSuite) SetupSuite() {
	s.App = Setup(s.T())
	admin := Token(s.T(), s.App)
	s.TokenA = createUserAsAdmin(s.T(), s.App, admin,
		`{"name":"dm-usera","email":"dma@test.sk","password":"kreslo"}`)
	_ = createUserAsAdmin(s.T(), s.App, admin,
		`{"name":"dm-userb","email":"dmb@test.sk","password":"kreslo"}`)
	s.IdB = idOfUser(s.T(), s.App, admin, "dmb@test.sk")
}

func (s *MessageDmSuite) TearDownSuite() {
	s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email IN ('dma@test.sk','dmb@test.sk')")
}

func (s *MessageDmSuite) Test_01_DM_WithoutSharedTeam_Succeeds() {
	body := fmt.Sprintf(`{"message":"hello","idRecipient":%d,"idMessageRecipientType":%d}`,
		s.IdB, model.TeammateRecipientType)
	res := Request(s.T(), s.App, "POST", "/api/private/message", body, s.TokenA)
	s.Equal(http.StatusOK, res.StatusCode)
}

func (s *MessageDmSuite) Test_02_DM_ToNonexistentUser_Fails() {
	body := fmt.Sprintf(`{"message":"hello","idRecipient":999999999,"idMessageRecipientType":%d}`,
		model.TeammateRecipientType)
	res := Request(s.T(), s.App, "POST", "/api/private/message", body, s.TokenA)
	s.Equal(http.StatusForbidden, res.StatusCode)
}

func (s *MessageDmSuite) Test_03_TeammatesEndpoint_Gone() {
	res := Request(s.T(), s.App, "GET", "/api/private/teammates", "", s.TokenA)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

func Test_RunMessageDmSuite(t *testing.T) {
	suite.Run(t, new(MessageDmSuite))
}
