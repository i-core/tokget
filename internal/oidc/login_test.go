/*
Copyright (c) JSC iCore.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package oidc

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"

	"github.com/i-core/tokget/internal/errors"
	"github.com/i-core/tokget/internal/log"
)

func TestLogin(t *testing.T) {
	var (
		// verbose turns on verbose logging in tests.
		verbose = os.Getenv("TOKGET_TEST_VERBOSE") == "true"
		// remoteChromeURL is a remote chrome's url to testing with a remote Chrome process.
		remoteChromeURL = os.Getenv("TOKGET_TEST_REMOTE_CHROME")
		// a test server's hostname that is accessible in a remote Chrome process.
		testServerHost = os.Getenv("TOKGET_TEST_SERVER_HOST")
	)

	type endpoint struct {
		path      string
		status    int
		redirect  string
		html      string
		wantQuery map[string]interface{}
		wantBody  interface{}
	}

	testCnf := &LoginConfig{
		ClientID:      "test-client",
		RedirectURI:   "http://localhost:9000/auth-callback",
		Scopes:        "openid profile email",
		Username:      "foo",
		Password:      "bar",
		UsernameField: "#user",
		PasswordField: "#pass",
		SubmitButton:  "#submit",
		ErrorMessage:  "#error",
	}
	testQuery := map[string]interface{}{
		"client_id":     "test-client",
		"response_type": "id_token token",
		"scope":         "openid profile email",
		"redirect_uri":  "http://localhost:9000/auth-callback",
		"state":         "12345678",
		"nonce":         "87654321",
	}

	errStr := func(err error) string {
		if v, ok := err.(*errors.Error); ok && v.Kind != errors.KindOther {
			return v.Kind.String()
		}
		return err.Error()
	}

	testCases := []struct {
		name         string
		endpoints    []endpoint
		cnf          *LoginConfig
		wantAccToken string
		wantIDToken  string
		wantErr      error
	}{
		{
			name:    "endpoint is missed",
			wantErr: errors.New(errors.KindEndpointMissed),
		},
		{
			name: "clientID is missed",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			wantErr: errors.New(errors.KindClientIDMissed),
		},
		{
			name: "client's redirect uri is missed",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf: &LoginConfig{
				ClientID: "test-client",
			},
			wantErr: errors.New(errors.KindRedirectURIMissed),
		},
		{
			name: "scopes are missed",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf: &LoginConfig{
				ClientID:    "test-client",
				RedirectURI: "http://localhost:9000/auth-callback",
			},
			wantErr: errors.New(errors.KindScopesMissed),
		},
		{
			name: "username is missed",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf: &LoginConfig{
				ClientID:    "test-client",
				RedirectURI: "http://localhost:9000/auth-callback",
				Scopes:      "openid profile email",
			},
			wantErr: errors.New(errors.KindUsernameMissed),
		},
		{
			name: "username field's selector is missed",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf: &LoginConfig{
				ClientID:    "test-client",
				RedirectURI: "http://localhost:9000/auth-callback",
				Scopes:      "openid profile email",
				Username:    "foo",
				Password:    "bar",
			},
			wantErr: errors.New(errors.KindUsernameFieldMissed),
		},
		{
			name: "password field's selector is missed",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf: &LoginConfig{
				ClientID:      "test-client",
				RedirectURI:   "http://localhost:9000/auth-callback",
				Scopes:        "openid profile email",
				Username:      "foo",
				Password:      "bar",
				UsernameField: "#user",
			},
			wantErr: errors.New(errors.KindPasswordFieldMissed),
		},
		{
			name: "submit button's selector is missed",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf: &LoginConfig{
				ClientID:      "test-client",
				RedirectURI:   "http://localhost:9000/auth-callback",
				Scopes:        "openid profile email",
				Username:      "foo",
				Password:      "bar",
				UsernameField: "#user",
				PasswordField: "#pass",
			},
			wantErr: errors.New(errors.KindSubmitButtonMissed),
		},
		{
			name: "error message's selector is missed",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf: &LoginConfig{
				ClientID:      "test-client",
				RedirectURI:   "http://localhost:9000/auth-callback",
				Scopes:        "openid profile email",
				Username:      "foo",
				Password:      "bar",
				UsernameField: "#user",
				PasswordField: "#pass",
				SubmitButton:  "#submit",
			},
			wantErr: errors.New(errors.KindErrorMessageMissed),
		},
		{
			name: "login page error: invalid client id",
			endpoints: []endpoint{
				{
					path:     "/oauth2/auth",
					status:   http.StatusPermanentRedirect,
					redirect: "/error?error=invalid client id&error_description=invalid client id desc",
				},
				{
					path:   "/error",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf: &LoginConfig{
				ClientID:      "test-client",
				RedirectURI:   "http://localhost:9000/auth-callback",
				Scopes:        "openid profile email",
				Username:      "foo",
				Password:      "bar",
				UsernameField: "#user",
				PasswordField: "#pass",
				SubmitButton:  "#submit",
				ErrorMessage:  "#error",
			},
			wantErr: errors.New(errors.KindOIDCError),
		},
		{
			name: "login page without username field",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf:     testCnf,
			wantErr: errors.New(errors.KindUsernameFieldInvalid),
		},
		{
			name: "login page without password field",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html: `
						<html>
							<body>
								<form>
									<input id="user"/>
								</form>
							</body>
						</html>
					`,
				},
			},
			cnf:     testCnf,
			wantErr: errors.New(errors.KindPasswordFieldInvalid),
		},
		{
			name: "login page without submit button",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html: `
						<html>
							<body>
								<form>
									<input id="user"/>
									<input id="pass"/>
								</form>
							</body>
						</html>
					`,
				},
			},
			cnf:     testCnf,
			wantErr: errors.New(errors.KindSubmitButtonInvalid),
		},
		{
			name: "openid connect error after submit",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html:   htmlForm("/handle-auth"),
				},
				{
					path:     "/handle-auth",
					status:   http.StatusPermanentRedirect,
					redirect: "/error?error=authentication failed&error_description=authentication failed",
				},
				{
					path:   "/error",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf:     testCnf,
			wantErr: errors.New(errors.KindOIDCError),
		},
		{
			name: "invalid password",
			endpoints: []endpoint{
				{
					path:      "/oauth2/auth",
					wantQuery: testQuery,
					status:    http.StatusOK,
					html:      htmlForm("/handle-auth"),
				},
				{
					path:     "/handle-auth",
					status:   http.StatusOK,
					html:     htmlFormWithError("/handle-auth", "invalid password"),
					wantBody: map[string]interface{}{"user": "foo", "pass": "bar"},
				},
			},
			cnf:     testCnf,
			wantErr: errors.New(errors.KindLoginError),
		},
		{
			name: "unexpected error page after submit",
			endpoints: []endpoint{
				{
					path:   "/oauth2/auth",
					status: http.StatusOK,
					html:   htmlForm("/handle-auth"),
				},
				{
					path:   "/handle-auth",
					status: http.StatusOK,
					html:   htmlForm("/handle-auth"),
				},
			},
			cnf:     testCnf,
			wantErr: errors.New(errors.KindLoginError),
		},
		{
			name: "happy path",
			endpoints: []endpoint{
				{
					path:      "/oauth2/auth",
					wantQuery: testQuery,
					status:    http.StatusOK,
					html:      htmlForm("/handle-auth"),
				},
				{
					path:     "/handle-auth",
					status:   http.StatusPermanentRedirect,
					redirect: "http://localhost:3000#access_token=access_token_value&id_token=id_token_value",
					wantBody: map[string]interface{}{"user": "foo", "pass": "bar"},
				},
			},
			cnf:          testCnf,
			wantAccToken: "access_token_value",
			wantIDToken:  "id_token_value",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cnf := tc.cnf
			if cnf == nil {
				cnf = &LoginConfig{}
			}
			if tc.endpoints != nil {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var (
						ep       endpoint
						epExists bool
					)
					for _, v := range tc.endpoints {
						if v.path == r.URL.Path {
							ep = v
							epExists = true
							break
						}
					}
					if !epExists {
						http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
						return
					}

					if ep.wantQuery != nil {
						query := make(map[string]interface{})
						for param := range r.URL.Query() {
							query[param] = r.URL.Query().Get(param)
						}
						if !reflect.DeepEqual(query, ep.wantQuery) {
							t.Fatalf("got query %#v, want query: %#v", query, ep.wantQuery)
						}
					}

					if ep.wantBody != nil {
						var body map[string]interface{}
						if r.Body != http.NoBody {
							b, err := ioutil.ReadAll(r.Body)
							if err != nil {
								t.Fatalf("failed to decode login form's data: %s", err)
							}
							q, err := url.ParseQuery(string(b))
							if err != nil {
								t.Fatalf("failed to decode login form's data: %s", err)
							}
							body = make(map[string]interface{})
							for key := range q {
								body[key] = q.Get(key)
							}
						}
						if !reflect.DeepEqual(body, ep.wantBody) {
							t.Fatalf("got form data %#v, want form data: %#v", body, ep.wantBody)
						}
					}

					if ep.status >= 300 && ep.status < 400 {
						http.Redirect(w, r, ep.redirect, ep.status)
						return
					}
					w.Header().Set("Content-Type", "text/html")
					w.WriteHeader(ep.status)
					fmt.Fprintln(w, ep.html)
				}))
				defer srv.Close()

				cnf.Endpoint = srv.URL
				if testServerHost != "" {
					u, err := url.Parse(srv.URL)
					if err != nil {
						t.Fatalf("failed to parse test endpoint's URL: %s", err)
					}
					u.Host = fmt.Sprintf("%s:%s", testServerHost, u.Port())
					cnf.Endpoint = u.String()
				}
			}

			ctx := log.WithLogger(context.Background(), verbose)
			got, err := Login(ctx, remoteChromeURL, cnf)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("\ngot no errors\nwant error:\n\t%s", errStr(tc.wantErr))
				}
				if !errors.Match(err, tc.wantErr) {
					t.Fatalf("\ngot error:\n\t%s\nwant error:\n\t%s", errStr(err), errStr(tc.wantErr))
				}
				return
			}

			if err != nil {
				t.Fatalf("\ngot error:\n\t%s\nwant no errors", errStr(err))
			}

			want := &LoginData{AccessToken: tc.wantAccToken, IDToken: tc.wantIDToken}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v, want %#v", got, want)
			}
		})
	}
}

func htmlForm(action string) string {
	return `
		<html>
			<body>
				<form method="post" action="` + action + `">
					<input id="user" name="user"/>
					<input id="pass" name="pass"/>
					<button id="submit">login</button>
				</form>
			</body>
		</html>
	`
}

func htmlFormWithError(action, err string) string {
	return `
		<html>
			<body>
				<form method="post" action="` + action + `">
					<input id="user" name="user"/>
					<input id="pass" name="pass"/>
					<button id="submit">login</button>
					<div id="error">` + err + `</div>
				</form>
			</body>
		</html>
	`
}
