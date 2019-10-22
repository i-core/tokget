/*
Copyright (c) JSC iCore.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package oidc

import (
	"context"
	"net/url"

	"github.com/i-core/tokget/internal/chrome"
	"github.com/i-core/tokget/internal/errors"
	"github.com/i-core/tokget/internal/log"
)

// LogoutConfig is a configuration of the logout process.
type LogoutConfig struct {
	Endpoint string `json:"endpoint"` // an OpenID Connect endpoint
	IDToken  string `json:"idToken"`  // an ID token
}

// Logout logs a user out and revoke the specified ID token.
func Logout(ctx context.Context, chromeURL string, cnf *LogoutConfig) error {
	//
	// Step 1. Validate input parameters.
	//
	if cnf.Endpoint == "" {
		return errors.New(errors.KindEndpointMissed, "OpenID Connect endpoint is missed")
	}
	endpoint, err := url.Parse(cnf.Endpoint)
	if err != nil {
		return errors.New(errors.KindEndpointInvalid, "OpenID Connect endpoint has an invalid value")
	}
	if cnf.IDToken == "" {
		return errors.New(errors.KindIDTokenMissed, "ID token is missed")
	}

	//
	// Step 2. Initialize Chrome connection.
	//
	var cancel context.CancelFunc
	if ctx, cancel, err = chrome.ConnectWithContext(ctx, chromeURL, chrome.DomainNetwork); err != nil {
		return errors.Wrap(err, "connect to chrome")
	}
	defer cancel()

	navHistory, err := chrome.NewNavHistory(ctx)
	if err != nil {
		return errors.Wrap(err, "initialize navigation history")
	}

	//
	// Step 3. Navigate to the OpenID Connect Provider's logout page, and process result.
	//
	logoutURL := buildLogoutURL(endpoint, cnf.IDToken)
	logger := log.LoggerFromContext(ctx).Sugar()
	logger.Debugf("Navigate to the logout page %q", logoutURL)
	if err = chrome.Navigate(ctx, logoutURL); err != nil {
		return errors.Wrap(err, "navigate to the logout page")
	}
	if err = extractOIDCError(navHistory.Last()); err != nil {
		return err
	}
	logger.Debug(`Logged out`)
	return nil
}

func buildLogoutURL(endpoint *url.URL, idToken string) string {
	ref, err := url.Parse("/oauth2/sessions/logout")
	if err != nil {
		panic(errors.Wrap(err, "make logout url"))
	}
	loURL := endpoint.ResolveReference(ref)
	query := loURL.Query()
	query.Set("id_token_hint", idToken)
	query.Set("state", "12345678")
	loURL.RawQuery = query.Encode()
	return loURL.String()
}
