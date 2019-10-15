package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/i-core/tokget/internal/errors"
	"github.com/i-core/tokget/internal/oidc"
	"go.uber.org/zap"
)

var testLogFn = func(ctx context.Context) *zap.Logger { return zap.NewNop() }

func TestLoginHandler(t *testing.T) {
	testCases := []struct {
		name       string
		defaults   Defaults
		body       io.Reader
		loginErr   error
		tokens     *oidc.LoginData
		wantConfig *oidc.LoginConfig
		wantStatus int
		wantTokens *oidc.LoginData
	}{
		{
			name:       "substitute the default RedirectURI",
			defaults:   Defaults{RedirectURI: "test"},
			tokens:     &oidc.LoginData{AccessToken: "test", IDToken: "test"},
			wantConfig: &oidc.LoginConfig{RedirectURI: "test"},
			wantStatus: http.StatusOK,
			wantTokens: &oidc.LoginData{AccessToken: "test", IDToken: "test"},
		},
		{
			name:       "substitute the default Scopes",
			defaults:   Defaults{Scopes: "test"},
			tokens:     &oidc.LoginData{AccessToken: "test", IDToken: "test"},
			wantConfig: &oidc.LoginConfig{Scopes: "test"},
			wantStatus: http.StatusOK,
			wantTokens: &oidc.LoginData{AccessToken: "test", IDToken: "test"},
		},
		{
			name:       "substitute the default UsernameField",
			defaults:   Defaults{UsernameField: "test"},
			tokens:     &oidc.LoginData{AccessToken: "test", IDToken: "test"},
			wantConfig: &oidc.LoginConfig{UsernameField: "test"},
			wantStatus: http.StatusOK,
			wantTokens: &oidc.LoginData{AccessToken: "test", IDToken: "test"},
		},
		{
			name:       "substitute the default PasswordField",
			defaults:   Defaults{PasswordField: "test"},
			tokens:     &oidc.LoginData{AccessToken: "test", IDToken: "test"},
			wantConfig: &oidc.LoginConfig{PasswordField: "test"},
			wantStatus: http.StatusOK,
			wantTokens: &oidc.LoginData{AccessToken: "test", IDToken: "test"},
		},
		{
			name:       "substitute the default SubmitButton",
			defaults:   Defaults{SubmitButton: "test"},
			tokens:     &oidc.LoginData{AccessToken: "test", IDToken: "test"},
			wantConfig: &oidc.LoginConfig{SubmitButton: "test"},
			wantStatus: http.StatusOK,
			wantTokens: &oidc.LoginData{AccessToken: "test", IDToken: "test"},
		},
		{
			name:       "substitute the default ErrorMessage",
			defaults:   Defaults{ErrorMessage: "test"},
			tokens:     &oidc.LoginData{AccessToken: "test", IDToken: "test"},
			wantConfig: &oidc.LoginConfig{ErrorMessage: "test"},
			wantStatus: http.StatusOK,
			wantTokens: &oidc.LoginData{AccessToken: "test", IDToken: "test"},
		},
		{
			name: "login parameters are transfered to the login function",
			body: toJSON(oidc.LoginConfig{
				Endpoint:      "test-endpoint",
				ClientID:      "test-client-id",
				RedirectURI:   "test-redirect-uri",
				Scopes:        "test-scopes",
				Username:      "test-username",
				Password:      "test-password",
				UsernameField: "test-username-field",
				PasswordField: "test-password-field",
				SubmitButton:  "test-submit-button",
				ErrorMessage:  "test-error-message",
			}),
			tokens: &oidc.LoginData{AccessToken: "test", IDToken: "test"},
			wantConfig: &oidc.LoginConfig{
				Endpoint:      "test-endpoint",
				ClientID:      "test-client-id",
				RedirectURI:   "test-redirect-uri",
				Scopes:        "test-scopes",
				Username:      "test-username",
				Password:      "test-password",
				UsernameField: "test-username-field",
				PasswordField: "test-password-field",
				SubmitButton:  "test-submit-button",
				ErrorMessage:  "test-error-message",
			},
			wantStatus: http.StatusOK,
			wantTokens: &oidc.LoginData{AccessToken: "test", IDToken: "test"},
		},
		{
			name:       "bad request when endpoint is missed",
			loginErr:   errors.New(errors.KindEndpointMissed),
			wantConfig: &oidc.LoginConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when client ID is missed",
			loginErr:   errors.New(errors.KindClientIDMissed),
			wantConfig: &oidc.LoginConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when redirect URI is missed",
			loginErr:   errors.New(errors.KindRedirectURIMissed),
			wantConfig: &oidc.LoginConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when scopes are missed",
			loginErr:   errors.New(errors.KindScopesMissed),
			wantConfig: &oidc.LoginConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when username is missed",
			loginErr:   errors.New(errors.KindUsernameMissed),
			wantConfig: &oidc.LoginConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when username field is missed",
			loginErr:   errors.New(errors.KindUsernameFieldMissed),
			wantConfig: &oidc.LoginConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when password field is missed",
			loginErr:   errors.New(errors.KindPasswordFieldMissed),
			wantConfig: &oidc.LoginConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when submit button is missed",
			loginErr:   errors.New(errors.KindSubmitButtonMissed),
			wantConfig: &oidc.LoginConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when error message is missed",
			loginErr:   errors.New(errors.KindErrorMessageMissed),
			wantConfig: &oidc.LoginConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when endpoint is invalid",
			loginErr:   errors.New(errors.KindEndpointInvalid),
			wantConfig: &oidc.LoginConfig{},
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var gotConfig *oidc.LoginConfig
			testLoginFn := func(ctx context.Context, cnf *oidc.LoginConfig) (*oidc.LoginData, error) {
				gotConfig = cnf
				if tc.loginErr != nil {
					return nil, tc.loginErr
				}
				return tc.tokens, nil
			}
			h := newLoginHandler(tc.defaults, testLoginFn, testLogFn)

			req := httptest.NewRequest(http.MethodPost, "/", tc.body)
			res := httptest.NewRecorder()
			h.ServeHTTP(res, req)

			var gotTokens *oidc.LoginData
			if res.Body.Len() > 0 {
				gotTokens = &oidc.LoginData{}
				if err := json.Unmarshal(res.Body.Bytes(), gotTokens); err != nil {
					t.Fatalf("failed to read the login handler's response: %s", err)
				}
			}

			if !reflect.DeepEqual(gotConfig, tc.wantConfig) {
				t.Errorf("got config %#v; want config: %#v", gotConfig, tc.wantConfig)
			}
			if res.Code != tc.wantStatus {
				t.Errorf("got status code %d; want status code: %#v", res.Code, tc.wantStatus)
			}
			if tc.wantStatus == http.StatusOK {
				if !reflect.DeepEqual(gotTokens, tc.wantTokens) {
					t.Errorf("got tokens %#v; want tokens: %#v", gotTokens, tc.wantTokens)
				}
			}
		})
	}
}

func TestLogoutHandler(t *testing.T) {
	testCases := []struct {
		name       string
		body       io.Reader
		loginErr   error
		wantConfig *oidc.LogoutConfig
		wantStatus int
	}{
		{
			name: "logout parameters are transfered to the login function",
			body: toJSON(oidc.LogoutConfig{
				Endpoint: "test-endpoint",
				IDToken:  "test-id-token",
			}),
			wantConfig: &oidc.LogoutConfig{
				Endpoint: "test-endpoint",
				IDToken:  "test-id-token",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "bad request when endpoint is missed",
			loginErr:   errors.New(errors.KindEndpointMissed),
			wantConfig: &oidc.LogoutConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when client ID is missed",
			loginErr:   errors.New(errors.KindClientIDMissed),
			wantConfig: &oidc.LogoutConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when redirect URI is missed",
			loginErr:   errors.New(errors.KindRedirectURIMissed),
			wantConfig: &oidc.LogoutConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when scopes are missed",
			loginErr:   errors.New(errors.KindScopesMissed),
			wantConfig: &oidc.LogoutConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when username is missed",
			loginErr:   errors.New(errors.KindUsernameMissed),
			wantConfig: &oidc.LogoutConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when username field is missed",
			loginErr:   errors.New(errors.KindUsernameFieldMissed),
			wantConfig: &oidc.LogoutConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when password field is missed",
			loginErr:   errors.New(errors.KindPasswordFieldMissed),
			wantConfig: &oidc.LogoutConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when submit button is missed",
			loginErr:   errors.New(errors.KindSubmitButtonMissed),
			wantConfig: &oidc.LogoutConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when error message is missed",
			loginErr:   errors.New(errors.KindErrorMessageMissed),
			wantConfig: &oidc.LogoutConfig{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "bad request when endpoint is invalid",
			loginErr:   errors.New(errors.KindEndpointInvalid),
			wantConfig: &oidc.LogoutConfig{},
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var gotConfig *oidc.LogoutConfig
			testLogoutFn := func(ctx context.Context, cnf *oidc.LogoutConfig) error {
				gotConfig = cnf
				return tc.loginErr
			}
			h := newLogoutHandler(testLogoutFn, testLogFn)

			req := httptest.NewRequest(http.MethodPost, "/", tc.body)
			res := httptest.NewRecorder()
			h.ServeHTTP(res, req)

			if !reflect.DeepEqual(gotConfig, tc.wantConfig) {
				t.Errorf("got config %#v; want config: %#v", gotConfig, tc.wantConfig)
			}
			if res.Code != tc.wantStatus {
				t.Errorf("got status code %d; want status code: %#v", res.Code, tc.wantStatus)
			}
		})
	}
}

func toJSON(data interface{}) io.Reader {
	b, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(b)
}
