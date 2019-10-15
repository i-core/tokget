/*
Copyright (c) JSC iCore.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package oidc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"

	"github.com/i-core/tokget/internal/errors"
	"github.com/i-core/tokget/internal/log"
)

func TestLogout(t *testing.T) {
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
	}

	testCases := []struct {
		name      string
		endpoints []endpoint
		cnf       *LogoutConfig
		wantErr   error
	}{
		{
			name:    "endpoint is missed",
			wantErr: errors.New(errors.KindEndpointMissed),
		},
		{
			name: "id token is missed",
			endpoints: []endpoint{
				{
					path:   "/oauth2/sessions/logout",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			wantErr: errors.New(errors.KindIDTokenMissed),
		},
		{
			name: "logout error",
			endpoints: []endpoint{
				{
					path:      "/oauth2/sessions/logout",
					wantQuery: map[string]interface{}{"id_token_hint": "id-token", "state": "12345678"},
					status:    http.StatusPermanentRedirect,
					redirect:  "/error?error=logout error&error_description=logout error",
				},
				{
					path:   "/error",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf:     &LogoutConfig{IDToken: "id-token"},
			wantErr: errors.New(errors.KindOIDCError),
		},
		{
			name: "happy path",
			endpoints: []endpoint{
				{
					path:      "/oauth2/sessions/logout",
					wantQuery: map[string]interface{}{"id_token_hint": "id-token", "state": "12345678"},
					status:    http.StatusPermanentRedirect,
					redirect:  "/post-logout",
				},
				{
					path:   "/post-logout",
					status: http.StatusOK,
					html:   "<html><body></body></html>",
				},
			},
			cnf: &LogoutConfig{IDToken: "id-token"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cnf := tc.cnf
			if cnf == nil {
				cnf = &LogoutConfig{}
			}
			if len(tc.endpoints) > 0 {
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
			err := Logout(ctx, remoteChromeURL, cnf)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("\ngot no errors\nwant error:\n\t%s", tc.wantErr)
				}
				if !errors.Match(err, tc.wantErr) {
					t.Fatalf("\ngot error:\n\t%s\nwant error:\n\t%s", err, tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("\ngot error:\n\t%s\nwant no errors", err)
			}
		})
	}
}
