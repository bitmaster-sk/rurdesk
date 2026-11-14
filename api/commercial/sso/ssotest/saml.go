package ssotest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
)

var (
	formAction = regexp.MustCompile(`<form[^>]*action="([^"]*)"`)
	formInput  = regexp.MustCompile(`<input[^>]*name="([^"]*)"[^>]*value="([^"]*)"`)
)

type SamlIdp struct {
	server     *httptest.Server
	idp        *saml.IdentityProvider
	mu         sync.Mutex
	spMetadata *saml.EntityDescriptor
	session    *saml.Session
}

func NewSamlIdp() (*SamlIdp, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generating saml idp key: %w", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test saml idp"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("creating saml idp certificate: %w", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parsing saml idp certificate: %w", err)
	}
	fake := &SamlIdp{}
	mux := http.NewServeMux()
	fake.server = httptest.NewServer(mux)
	metadataUrl, _ := url.Parse(fake.server.URL + "/metadata")
	ssoUrl, _ := url.Parse(fake.server.URL + "/sso")
	fake.idp = &saml.IdentityProvider{
		Key:                     key,
		Certificate:             certificate,
		MetadataURL:             *metadataUrl,
		SSOURL:                  *ssoUrl,
		ServiceProviderProvider: fake,
		SessionProvider:         fake,
	}
	mux.HandleFunc("GET /metadata", fake.idp.ServeMetadata)
	mux.HandleFunc("GET /sso", fake.idp.ServeSSO)
	return fake, nil
}

func (s *SamlIdp) Close() {
	s.server.Close()
}

func (s *SamlIdp) MetadataUrl() string {
	return s.server.URL + "/metadata"
}

func (s *SamlIdp) EntityId() string {
	return s.MetadataUrl()
}

func (s *SamlIdp) MetadataXml() (string, error) {
	metadata, err := xml.Marshal(s.idp.Metadata())
	if err != nil {
		return "", fmt.Errorf("encoding saml idp metadata: %w", err)
	}
	return string(metadata), nil
}

func (s *SamlIdp) TrustServiceProvider(metadataXml []byte) error {
	descriptor, err := samlsp.ParseMetadata(metadataXml)
	if err != nil {
		return fmt.Errorf("parsing sp metadata: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spMetadata = descriptor
	return nil
}

func (s *SamlIdp) StopEncrypting() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.spMetadata.SPSSODescriptors {
		descriptor := &s.spMetadata.SPSSODescriptors[i]
		signing := descriptor.KeyDescriptors[:0]
		for _, key := range descriptor.KeyDescriptors {
			if key.Use != "encryption" {
				signing = append(signing, key)
			}
		}
		descriptor.KeyDescriptors = signing
	}
}

func (s *SamlIdp) GetServiceProvider(_ *http.Request, _ string) (*saml.EntityDescriptor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.spMetadata == nil {
		return nil, os.ErrNotExist
	}
	return s.spMetadata, nil
}

func (s *SamlIdp) GetSession(w http.ResponseWriter, _ *http.Request, _ *saml.IdpAuthnRequest) *saml.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		http.Error(w, "no session", http.StatusForbidden)
		return nil
	}
	return s.session
}

type SamlUser struct {
	NameId       string
	NameIdFormat string
	Attributes   map[string][]string
}

type SamlPost struct {
	AcsUrl string
	Form   url.Values
}

func (s *SamlIdp) signIn(user SamlUser) {
	session := &saml.Session{
		ID:           fmt.Sprintf("session-%d", time.Now().UnixNano()),
		CreateTime:   time.Now(),
		ExpireTime:   time.Now().Add(time.Hour),
		Index:        "1",
		NameID:       user.NameId,
		NameIDFormat: user.NameIdFormat,
	}
	for name, values := range user.Attributes {
		attribute := saml.Attribute{Name: name, NameFormat: "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"}
		for _, value := range values {
			attribute.Values = append(attribute.Values, saml.AttributeValue{Type: "xs:string", Value: value})
		}
		session.CustomAttributes = append(session.CustomAttributes, attribute)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.session = session
}

func (s *SamlIdp) Authorize(authUrl string, user SamlUser) (SamlPost, error) {
	s.signIn(user)
	res, err := http.Get(authUrl)
	if err != nil {
		return SamlPost{}, fmt.Errorf("calling saml idp: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return SamlPost{}, fmt.Errorf("reading saml idp response: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return SamlPost{}, fmt.Errorf("saml idp answered %d: %s", res.StatusCode, body)
	}
	return parseResponseForm(body)
}

func parseResponseForm(body []byte) (SamlPost, error) {
	action := formAction.FindSubmatch(body)
	if action == nil {
		return SamlPost{}, errors.New("saml idp response has no form")
	}
	post := SamlPost{AcsUrl: html.UnescapeString(string(action[1])), Form: url.Values{}}
	for _, input := range formInput.FindAllSubmatch(body, -1) {
		post.Form.Set(string(input[1]), html.UnescapeString(string(input[2])))
	}
	return post, nil
}

func (s *SamlIdp) Unsolicited(spEntityId string, user SamlUser) (SamlPost, error) {
	s.signIn(user)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, s.server.URL+"/sso", nil)
	s.idp.ServeIDPInitiated(recorder, request, spEntityId, "")
	return parseResponseForm(recorder.Body.Bytes())
}
