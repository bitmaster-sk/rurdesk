package model

type SsoMethod struct {
	Protocol        string `json:"protocol"`
	IsEnforced      bool   `json:"isEnforced"`
	CanStaySignedIn bool   `json:"canStaySignedIn"`
}

type AuthMethodsRes struct {
	Password       bool       `json:"password"`
	Sso            *SsoMethod `json:"sso"`
	SsoUnavailable bool       `json:"ssoUnavailable"`
}
