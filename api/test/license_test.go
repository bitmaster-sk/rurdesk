package test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/bitmaster-sk/rurdesk/api/internal/injector"
	"github.com/bitmaster-sk/rurdesk/api/internal/issue"
	"github.com/bitmaster-sk/rurdesk/api/internal/license"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type stubLicense struct {
	status   license.Status
	features []license.Feature
}

func (s stubLicense) IsAvailable(_ context.Context, feature license.Feature) bool {
	for _, entitled := range s.features {
		if entitled == feature {
			return true
		}
	}
	return false
}

func (s stubLicense) Status(context.Context) license.Status {
	return s.status
}

// rebuildWithLicense swaps the entitlement source and rewires the licence
// controller, which captures the provider when issue.New() binds the routes.
func rebuildWithLicense(t *testing.T, provider license.Provider) *issue.Application {
	injector.SetLicenseProvider(provider)
	for _, key := range []string{"license-controller", "router", "http-server"} {
		injector.Clear(key)
	}
	app, err := issue.New()
	require.Nil(t, err)
	return app
}

func licensedIn(days float64) license.Status {
	expiresAt := time.Now().Add(time.Duration(days * float64(24*time.Hour)))
	return license.Status{IsLicensed: true, Tenant: "acme", ExpiresAt: &expiresAt}
}

type licenseBody struct {
	Severity      string     `json:"severity"`
	DaysRemaining int        `json:"daysRemaining"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	IsDismissible bool       `json:"isDismissible"`
}

// LicenseSuite covers what GET /license tells the client: how close the licence
// is to expiring, and nothing else.
type LicenseSuite struct {
	suite.Suite
	App        *issue.Application
	AdminToken string
}

func (s *LicenseSuite) SetupSuite() {
	s.App = Setup(s.T())
	s.AdminToken = Token(s.T(), s.App)
}

func (s *LicenseSuite) TearDownSuite() {
	s.App = rebuildWithLicense(s.T(), license.Free{})
}

func (s *LicenseSuite) get(token string) licenseBody {
	res := Request(s.T(), s.App, "GET", "/api/private/license", "", token)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var body licenseBody
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&body))
	return body
}

func (s *LicenseSuite) Test_01_FreeInstance_NoWarning() {
	s.App = rebuildWithLicense(s.T(), license.Free{})
	s.AdminToken = Token(s.T(), s.App)

	s.Equal("none", s.get(s.AdminToken).Severity, "a free instance must never be shown a licence warning")
}

func (s *LicenseSuite) Test_02_LicensedFarFromExpiry_NoWarning() {
	s.App = rebuildWithLicense(s.T(), stubLicense{status: licensedIn(90)})
	s.AdminToken = Token(s.T(), s.App)

	body := s.get(s.AdminToken)
	s.Equal("none", body.Severity)
	s.Nil(body.ExpiresAt)
}

func (s *LicenseSuite) Test_03_SeesEscalatingWarning() {
	tests := []struct {
		days          float64
		severity      string
		isDismissible bool
	}{
		{29, "info", true},
		{19, "warning", true},
		{9, "error", false},
		{-1, "expired", false},
	}

	for _, test := range tests {
		s.Run(test.severity, func() {
			s.App = rebuildWithLicense(s.T(), stubLicense{status: licensedIn(test.days)})
			s.AdminToken = Token(s.T(), s.App)

			body := s.get(s.AdminToken)
			s.Equal(test.severity, body.Severity)
			s.Equal(test.isDismissible, body.IsDismissible)
			s.Require().NotNil(body.ExpiresAt)
			s.False(body.ExpiresAt.IsZero())
		})
	}
}

func (s *LicenseSuite) Test_04_NonAdmin_SeesTheSameWarning() {
	s.App = rebuildWithLicense(s.T(), stubLicense{status: licensedIn(5)})
	s.AdminToken = Token(s.T(), s.App)

	member := createUserAsAdmin(s.T(), s.App, s.AdminToken,
		`{"name":"licensemember","email":"licensemember@test.sk","password":"kreslo"}`)
	defer s.App.Pool.Exec(context.Background(),
		"DELETE FROM users.user WHERE email = 'licensemember@test.sk'")

	memberBody := s.get(member)
	s.Equal("error", memberBody.Severity, "the admin who can renew is often not the one using the app")
	s.Equal(s.get(s.AdminToken), memberBody)
}

func (s *LicenseSuite) Test_05_Anonymous_Unauthorized() {
	res := Request(s.T(), s.App, "GET", "/api/private/license", "", "")
	s.Equal(http.StatusUnauthorized, res.StatusCode)
}

func (s *LicenseSuite) Test_06_ResponseCarriesNothingButTheExpiryState() {
	s.App = rebuildWithLicense(s.T(), stubLicense{status: licensedIn(5), features: []license.Feature{license.SSO}})
	s.AdminToken = Token(s.T(), s.App)

	res := Request(s.T(), s.App, "GET", "/api/private/license", "", s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var body map[string]any
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&body))
	s.NotContains(body, "features", "which features are paid is documentation, not an API response")
	s.NotContains(body, "plan")
	s.NotContains(body, "tenant")
	s.Len(body, 4)
}

func (s *LicenseSuite) Test_07_SettingsDoesNotCarryLicense() {
	res := Request(s.T(), s.App, "GET", "/api/private/settings", "", s.AdminToken)
	s.Require().Equal(http.StatusOK, res.StatusCode)

	var body map[string]any
	s.Require().NoError(json.NewDecoder(res.Body).Decode(&body))
	s.NotContains(body, "license", "entitlement state has its own endpoint")
}

func Test_RunLicenseSuite(t *testing.T) {
	suite.Run(t, new(LicenseSuite))
}
