/*
Copyright (c) JSC iCore.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package oidc

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/i-core/tokget/internal/chrome"
	"github.com/i-core/tokget/internal/errors"
	"github.com/i-core/tokget/internal/log"
)

// LoginConfig is a configuration of the login process.
type LoginConfig struct {
	Endpoint      string `json:"endpoint"`      // an OpenID Connect endpoint
	ClientID      string `json:"clientId"`      // a client's ID
	RedirectURI   string `json:"redirectUri"`   // a client's redirect uri
	Scopes        string `json:"scopes"`        // OpenID Connect scopes
	Username      string `json:"username"`      // a user's name
	Password      string `json:"password"`      // a user's password
	UsernameField string `json:"usernameField"` // a CSS selector of the username field on the login form
	PasswordField string `json:"passwordField"` // a CSS selector of the password field on the login form
	SubmitButton  string `json:"submitButton"`  // a CSS selector of the submit button on the login form
	ErrorMessage  string `json:"errorMessage"`  // a CSS selector of an error message on the login form
}

// LoginData is a successful result of the login process.
type LoginData struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

// Login authenticates a user by opening the login page of an OpenID Connect Provider,
// and emulating user's actions to fill the authentication parameters and clicking the login button.
// The function returns a struct that contains an access token an ID token of the authenticated user.
func Login(ctx context.Context, chromeURL string, cnf *LoginConfig) (*LoginData, error) {
	logger := log.LoggerFromContext(ctx).Sugar()

	//
	// Step 1. Validate input parameters, and request a user for a password if it is not defined.
	//
	checks := []struct {
		param string
		kind  errors.Kind
		msg   string
	}{
		{
			param: cnf.Endpoint,
			kind:  errors.KindEndpointMissed,
			msg:   "OpenID Connect endpoint is missed",
		},
		{
			param: cnf.ClientID,
			kind:  errors.KindClientIDMissed,
			msg:   "client ID is missed",
		},
		{
			param: cnf.RedirectURI,
			kind:  errors.KindRedirectURIMissed,
			msg:   "client's redirect uri is missed",
		},
		{
			param: cnf.Scopes,
			kind:  errors.KindScopesMissed,
			msg:   "OpenID Connect scopes are missed",
		},
		{
			param: cnf.Username,
			kind:  errors.KindUsernameMissed,
			msg:   "username is missed",
		},
		{
			param: cnf.UsernameField,
			kind:  errors.KindUsernameFieldMissed,
			msg:   "username field's selector is missed",
		},
		{
			param: cnf.PasswordField,
			kind:  errors.KindPasswordFieldMissed,
			msg:   "password field's selector is missed",
		},
		{
			param: cnf.SubmitButton,
			kind:  errors.KindSubmitButtonMissed,
			msg:   "submit button's selector is missed",
		},
		{
			param: cnf.ErrorMessage,
			kind:  errors.KindErrorMessageMissed,
			msg:   "error message's selector is missed",
		},
	}
	for _, chk := range checks {
		if chk.param == "" {
			return nil, errors.New(chk.kind, chk.msg)
		}
	}

	endpoint, err := url.Parse(cnf.Endpoint)
	if err != nil {
		return nil, errors.New(errors.KindEndpointInvalid, "OpenID Connect endpoint has an invalid value")
	}

	//
	// Step 2. Initialize Chrome connection and open a new tab.
	//
	var cancelBrowser context.CancelFunc
	if ctx, cancelBrowser, err = chrome.ConnectWithContext(ctx, chromeURL, chrome.DomainNetwork, chrome.DomainRuntime); err != nil {
		return nil, errors.Wrap(err, "connect to chrome")
	}
	defer func() {
		logger.Debug("Disconnect Chrome")
		cancelBrowser()
	}()

	navHistory, err := chrome.NewNavHistory(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "initialize navigation history")
	}

	//
	// Step 3. Navigate to the OpenID Connect Provider's login page.
	//
	loginStartURL := buildLoginURL(endpoint, cnf.ClientID, cnf.RedirectURI, cnf.Scopes)
	logger.Debugf("Navigate to the login page %q\n", loginStartURL)
	if err = chrome.Navigate(ctx, loginStartURL); err != nil {
		return nil, errors.Wrap(err, "navigate to the login page")
	}
	if err = extractOIDCError(navHistory.Last()); err != nil {
		return nil, err
	}
	var loginPageContent string
	if err = chromedp.Run(ctx, chromedp.OuterHTML("html", &loginPageContent)); err != nil {
		return nil, errors.Wrap(err, "get the login page's content")
	}
	logger.Debugf("The login page is loaded:\n\n%s\n\n", loginPageContent)

	//
	// Step 4. Validate the login form.
	//
	// We expect that the login form contains the username field, password field and submit button.
	logger.Debug("Fill the login form")
	formHasElement := func(name, sel string, kind errors.Kind) error {
		has, hasErr := chrome.HasElement(ctx, sel)
		if hasErr != nil {
			return errors.Wrap(hasErr, "find %s", name)
		}
		if !has {
			return errors.New(kind, "the login form does not contains %s", name)
		}
		return nil
	}
	if err = formHasElement("the username field", cnf.UsernameField, errors.KindUsernameFieldInvalid); err != nil {
		return nil, err
	}
	if err = formHasElement("the password field", cnf.PasswordField, errors.KindPasswordFieldInvalid); err != nil {
		return nil, err
	}
	if err = formHasElement("the submit button", cnf.SubmitButton, errors.KindSubmitButtonInvalid); err != nil {
		return nil, err
	}

	//
	// Step 5. Fill the login form.
	//
	if err = chromedp.Run(ctx, chromedp.SendKeys(cnf.UsernameField, cnf.Username)); err != nil {
		return nil, errors.Wrap(err, "fill the username field")
	}
	if cnf.Password != "" {
		if err = chromedp.Run(ctx, chromedp.SendKeys(cnf.PasswordField, cnf.Password)); err != nil {
			return nil, errors.Wrap(err, "fill the password field")
		}
	}

	//
	// Step 6. Submit the login form.
	//
	logger.Debug("Submit the login form")
	// We submit the login form by clicking on the submit button instead of calling chromedp.Submit()
	// because of the tool emulates a user's actions.
	wait := chrome.PageLoadWaiterFunc(ctx, false, 5*time.Second)
	if err = chromedp.Run(ctx, chromedp.Click(cnf.SubmitButton)); err != nil {
		return nil, errors.Wrap(err, "submit the login form")
	}
	if err = wait(); err != nil {
		return nil, errors.Wrap(err, "wait for submiting the login form")
	}

	//
	// Step 7. Handle the submiting result.
	//
	// There are the next cases:
	// 1. The OpenID Connect Provider redirects a user to the client's redirect URI with tokens in the URL's fragment.
	// 2. The OpenID Connect Provider redirects a user to an OpenID Connect error's page.
	// 3. The OpenID Connect Provider shows a user the login page that contains authentication error's message.
	logger.Debug("Submiting is finished")
	postLoginURL := navHistory.Last()
	loginData, err := extractOIDCTokens(postLoginURL)
	if err != nil {
		return nil, errors.Wrap(err, "extract OpenID Connect tokens")
	}
	if loginData != nil {
		return loginData, nil
	}

	logger.Debug("Failed to authenticate the user")
	if err = extractOIDCError(postLoginURL); err != nil {
		return nil, err
	}
	errMsg, err := chrome.Text(ctx, cnf.ErrorMessage)
	if err != nil {
		return nil, errors.Wrap(err, "find submiting error message")
	}
	if errMsg != "" {
		return nil, errors.New(errors.KindLoginError, strings.TrimSpace(errMsg))
	}
	// There is an unexpected error page so just display the page's content to a user.
	var errPageContent string
	if err = chromedp.Run(ctx, chromedp.OuterHTML("html", &errPageContent)); err != nil {
		return nil, errors.Wrap(err, "read error page content")
	}
	return nil, errors.New(errors.KindLoginError, "unexpected error page %q\n%s", postLoginURL, errPageContent)
}

func buildLoginURL(endpoint *url.URL, clientID, redirectURI, scopes string) string {
	ref, err := url.Parse("/oauth2/auth")
	if err != nil {
		panic(errors.Wrap(err, "make login url"))
	}
	loginStartURL := endpoint.ResolveReference(ref)
	query := loginStartURL.Query()
	query.Set("client_id", clientID)
	query.Set("response_type", "id_token token")
	query.Set("scope", scopes)
	query.Set("redirect_uri", redirectURI)
	query.Set("state", "12345678")
	query.Set("nonce", "87654321")
	loginStartURL.RawQuery = query.Encode()
	return loginStartURL.String()
}

func extractOIDCTokens(u string) (*LoginData, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, errors.Wrap(err, "parse post login URL")
	}
	if parsedURL.Fragment == "" {
		return nil, nil
	}
	var userData url.Values
	if userData, err = url.ParseQuery(parsedURL.Fragment); err != nil {
		return nil, errors.Wrap(err, "parse the authentication callback's fragment")
	}
	accessToken := userData.Get("access_token")
	if accessToken == "" {
		return nil, errors.New("the authentication endpoint does not send an access token in the url's fragment")
	}
	idToken := userData.Get("id_token")
	if idToken == "" {
		return nil, errors.New("the authentication endpoint does not send an id token in the url's fragment")
	}
	return &LoginData{AccessToken: accessToken, IDToken: idToken}, nil
}
