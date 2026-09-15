package license

import "context"

type Feature string

const SSO Feature = "sso"

// Provider answers whether this instance is entitled to a paid feature.
type Provider interface {
	IsAvailable(ctx context.Context, feature Feature) bool
	Status(ctx context.Context) Status
}

// Free is the default of the open-source build. It entitles nothing and never
// calls out.
type Free struct{}

func (Free) IsAvailable(context.Context, Feature) bool {
	return false
}

func (Free) Status(context.Context) Status {
	return Status{}
}
