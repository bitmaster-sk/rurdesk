package policy

import (
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
)

func IsEnforced(settings config.Settings, isLicensed bool) bool {
	if !settings.IsEnabled || !settings.IsEnforced || !isLicensed {
		return false
	}
	_, err := config.ReadyBaseUrl()
	return err == nil
}
