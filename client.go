package apiai

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/cookiejar"

	"golang.org/x/oauth2"
)

// AuthType defines the type of authentication.
type AuthType string

const (
	AuthTypeNone         AuthType = "none"
	AuthTypeBasic                 = "basic"
	AuthTypeAPIKeyHeader          = "apikey-header"
	AuthTypeAPIKeyCookie          = "apikey-cookie"
	AuthTypeBearer                = "bearer"
	AuthTypeOAuth2                = "oauth2"
	AuthTypeCookie                = "cookie"
)

// AuthConfig contains the authentication configuration.
type AuthConfig struct {
	Type AuthType

	// Basic Auth
	Username string
	Password string

	// API Key
	APIKeyName  string // name of header or cookie
	APIKeyValue string

	// Bearer / OAuth2 / OpenID
	Token string

	// OAuth2 TokenSource (allows automatic token refresh)
	TokenSource oauth2.TokenSource

	// Cookie Auth: list of cookies (name=value)
	Cookies []*http.Cookie
}

// AuthRoundTripper wraps http.RoundTripper and adds authentication.
type AuthRoundTripper struct {
	transport http.RoundTripper
	config    *AuthConfig
}

// RoundTrip implements the RoundTripper interface.
func (art *AuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if art.config == nil || art.config.Type == AuthTypeNone {
		return art.transport.RoundTrip(req)
	}

	req = req.Clone(req.Context())

	switch art.config.Type {
	case AuthTypeBasic:
		req.SetBasicAuth(art.config.Username, art.config.Password)

	case AuthTypeAPIKeyHeader:
		if art.config.APIKeyName != "" && art.config.APIKeyValue != "" {
			req.Header.Set(art.config.APIKeyName, art.config.APIKeyValue)
		}

	case AuthTypeAPIKeyCookie:
		if art.config.APIKeyName != "" && art.config.APIKeyValue != "" {
			cookie := &http.Cookie{
				Name:  art.config.APIKeyName,
				Value: art.config.APIKeyValue,
			}
			req.AddCookie(cookie)
		}

	case AuthTypeBearer:
		if art.config.Token != "" {
			req.Header.Set("Authorization", "Bearer "+art.config.Token)
		}

	case AuthTypeOAuth2:
		var token *oauth2.Token
		var err error
		if art.config.TokenSource != nil {
			token, err = art.config.TokenSource.Token()
			if err != nil {
				return nil, fmt.Errorf("failed to get OAuth2 token: %w", err)
			}
		} else if art.config.Token != "" {
			token = &oauth2.Token{AccessToken: art.config.Token}
		} else {
			return nil, fmt.Errorf("OAuth2 requires either Token or TokenSource")
		}
		token.SetAuthHeader(req)

	case AuthTypeCookie:
		for _, cookie := range art.config.Cookies {
			req.AddCookie(cookie)
		}

	default:
		return nil, fmt.Errorf("unsupported auth type: %s", art.config.Type)
	}

	return art.transport.RoundTrip(req)
}

// NewHTTPClient creates an http.Client with the specified authentication.
// Usage examples
//
// Example 1: Basic Auth
// client1 := NewHTTPClient(&AuthConfig{
// 	Type:     AuthTypeBasic,
// 	Username: "admin",
// 	Password: "secret",
// })
//
// Example 2: API Key in header
// client2 := NewHTTPClient(&AuthConfig{
// 	Type:        AuthTypeAPIKeyHeader,
// 	APIKeyName:  "X-API-Key",
// 	APIKeyValue: "12345-abcde",
// })
//
// Example 3: Bearer Token
// client3 := NewHTTPClient(&AuthConfig{
// 	Type:  AuthTypeBearer,
// 	Token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
// })
//
// Example 4: OAuth2 with custom TokenSource
// oauth2Config := &oauth2.Config{...}
// ts := oauth2Config.TokenSource(context.Background(), token)
// client4 := NewHTTPClient(&AuthConfig{
// 	Type:         AuthTypeOAuth2,
// 	TokenSource:  nil, // can pass oauth2.TokenSource
// 	Token:        "access_token_here", // fallback
// })
//
// Example 5: Cookie Auth
// client5 := NewHTTPClient(&AuthConfig{
// 	Type: AuthTypeCookie,
// 	Cookies: []*http.Cookie{
// 		{Name: "sessionid", Value: "abc123"},
// 		{Name: "csrftoken", Value: "xyz789"},
// 	},
// })
//
// Use it
// resp, err := client1.Get("https://httpbin.org/basic-auth/admin/secret")
// if err != nil {
// 	panic(err)
// }
// defer resp.Body.Close()
// fmt.Println("Status:", resp.Status)
func NewHTTPClient(config *AuthConfig, opts ...func(*http.Client)) *http.Client {
	// Base transport (can be customized)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
	}

	// Add cookie jar if cookies are used
	var jar http.CookieJar
	if config.Type == AuthTypeAPIKeyCookie || config.Type == AuthTypeCookie {
		if j, err := cookiejar.New(nil); err == nil {
			jar = j
		}
	}

	client := &http.Client{
		Transport: &AuthRoundTripper{
			transport: transport,
			config:    config,
		},
		Jar: jar,
	}

	// Apply optional client settings
	for _, opt := range opts {
		opt(client)
	}

	return client
}
