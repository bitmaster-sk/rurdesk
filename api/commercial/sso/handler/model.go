package handler

import (
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/scim"
)

type ExchangeReq struct {
	Code string `json:"code" binding:"required,max=100"`
}

type ExchangeRes struct {
	Token     string `json:"token"`
	ReturnUrl string `json:"returnUrl"`
}

type StartTestRes struct {
	Url string `json:"url"`
}

type StateRes struct {
	*config.StateRes
	Scim scim.State `json:"scim"`
}

type SetScimEnabledReq struct {
	IsEnabled *bool `json:"isEnabled" binding:"required"`
}

type ScimTokenRes struct {
	Token string `json:"token"`
}
