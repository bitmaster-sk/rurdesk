package config

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
)

var (
	domainPattern        = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
	microsoftMultiTenant = []string{"common", "organizations", "consumers"}
)

func ValidateIssuer(raw string) (string, error) {
	issuer := strings.TrimSpace(raw)
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !isSecureOrLoopback(parsed) {
		return "", ErrIssuerInvalid
	}
	if strings.EqualFold(parsed.Hostname(), "login.microsoftonline.com") {
		tenant, _, _ := strings.Cut(strings.TrimPrefix(parsed.Path, "/"), "/")
		if slices.Contains(microsoftMultiTenant, strings.ToLower(tenant)) {
			return "", ErrIssuerMultiTenant
		}
	}
	return issuer, nil
}

func ValidatePublicBaseUrl(baseUrl string) error {
	parsed, err := url.Parse(baseUrl)
	if baseUrl == "" || err != nil || parsed.Host == "" || strings.Trim(parsed.Path, "/") != "" || !isSecureOrLoopback(parsed) {
		return ErrPublicUrlInvalid
	}
	return nil
}

func NormalizeDomains(domains []string) ([]string, error) {
	normalized := make([]string, 0, len(domains))
	for _, domain := range domains {
		value := strings.ToLower(strings.TrimSpace(domain))
		if value == "" {
			continue
		}
		if !domainPattern.MatchString(value) {
			return nil, ErrDomainInvalid
		}
		normalized = append(normalized, value)
	}
	slices.Sort(normalized)
	return slices.Compact(normalized), nil
}

func isSecureOrLoopback(parsed *url.URL) bool {
	switch parsed.Scheme {
	case "https":
		return true
	case "http":
		return isLoopbackHost(parsed.Hostname())
	}
	return false
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
