package identity

type Identity struct {
	Issuer            string
	Subject           string
	Email             string
	EmailVerified     *bool
	HostedDomain      string
	Name              string
	Groups            []string
	HasGroupsClaim    bool
	HasGroupsOverage  bool
	IsSubjectUnstable bool
}
