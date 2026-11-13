package samlauth

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/config"
	"github.com/bitmaster-sk/rurdesk/api/commercial/sso/identity"
	"github.com/crewjam/saml"
	dsig "github.com/russellhaering/goxmldsig"
)

const maxMetadataBytes = 2 << 20

var ErrNoRedirectBinding = errors.New("the identity provider metadata has no HTTP-Redirect single sign-on endpoint")

type Client struct {
	connection config.Connection
	sp         *saml.ServiceProvider
}

func NewClient(connection config.Connection, pair *config.KeyPair, baseUrl string, replay saml.AssertionReplayCache) (*Client, error) {
	descriptor, err := config.ParseIdpMetadata(connection.MetadataXml)
	if err != nil {
		return nil, err
	}
	sp, err := serviceProvider(pair, baseUrl)
	if err != nil {
		return nil, err
	}
	sp.IDPMetadata = descriptor
	sp.ReplayCache = replay
	connection.Issuer = descriptor.EntityID
	return &Client{connection: connection, sp: sp}, nil
}

func serviceProvider(pair *config.KeyPair, baseUrl string) (*saml.ServiceProvider, error) {
	metadataUrl, err := url.Parse(baseUrl + config.SpMetadataPath)
	if err != nil {
		return nil, fmt.Errorf("parsing sp metadata url: %w", err)
	}
	acsUrl, err := url.Parse(baseUrl + config.AcsPath)
	if err != nil {
		return nil, fmt.Errorf("parsing acs url: %w", err)
	}
	return &saml.ServiceProvider{
		EntityID:          metadataUrl.String(),
		Key:               pair.Key,
		Certificate:       pair.Certificate,
		MetadataURL:       *metadataUrl,
		AcsURL:            *acsUrl,
		AuthnNameIDFormat: saml.UnspecifiedNameIDFormat,
		SignatureMethod:   dsig.RSASHA256SignatureMethod,
	}, nil
}

type Request struct {
	Id      string
	request *saml.AuthnRequest
}

func (c *Client) NewRequest() (*Request, error) {
	location := c.sp.GetSSOBindingLocation(saml.HTTPRedirectBinding)
	if location == "" {
		return nil, ErrNoRedirectBinding
	}
	request, err := c.sp.MakeAuthenticationRequest(location, saml.HTTPRedirectBinding, saml.HTTPPostBinding)
	if err != nil {
		return nil, fmt.Errorf("making authn request: %w", err)
	}
	return &Request{Id: request.ID, request: request}, nil
}

func (c *Client) RedirectUrl(request *Request, relayState string) (string, error) {
	redirect, err := request.request.Redirect(relayState, c.sp)
	if err != nil {
		return "", fmt.Errorf("signing authn request: %w", err)
	}
	return redirect.String(), nil
}

func (c *Client) Identity(req *http.Request, requestId string) (identity.Identity, error) {
	assertion, err := c.sp.ParseResponse(req, []string{requestId})
	if err != nil {
		return identity.Identity{}, describe(err)
	}
	return IdentityFrom(c.connection, assertion)
}

func describe(err error) error {
	var invalid *saml.InvalidResponseError
	if errors.As(err, &invalid) && invalid.PrivateErr != nil {
		return fmt.Errorf("%w: %w", err, invalid.PrivateErr)
	}
	return err
}

func SpMetadata(pair *config.KeyPair, baseUrl string) ([]byte, error) {
	sp, err := serviceProvider(pair, baseUrl)
	if err != nil {
		return nil, err
	}
	metadata, err := xml.MarshalIndent(sp.Metadata(), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding sp metadata: %w", err)
	}
	return metadata, nil
}

func FetchMetadata(ctx context.Context, client *http.Client, metadataUrl string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataUrl, nil)
	if err != nil {
		return "", fmt.Errorf("building metadata request: %w", err)
	}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching idp metadata: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching idp metadata: status %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxMetadataBytes))
	if err != nil {
		return "", fmt.Errorf("reading idp metadata: %w", err)
	}
	if _, err := config.ParseIdpMetadata(string(body)); err != nil {
		return "", err
	}
	return string(body), nil
}
