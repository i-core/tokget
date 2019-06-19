/*
Copyright (c) JSC iCore.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package oidc

import (
	"fmt"
	"net/url"

	"github.com/i-core/tokget/internal/errors"
)

// extractOIDCError returns an error from OpenID Connect's error url.
//
// By spec OpenID Connect specification an error url contains query parameters "error" and "error_description".
// When the query parameter "error" is empty, the function returns nil.
// When the query parameter "error" is not empty, the function returns an error with a message equals to
// the query parameter "error_description".
//
// Ory Hydra server responds with an error url that also contains query parameter "error_hint". The function
// includes a value of the parameter to an error's message when the value is not empty.
func extractOIDCError(u string) error {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return errors.Wrap(err, "parse post login url")
	}
	if qerr := parsedURL.Query().Get("error"); qerr == "" {
		return nil
	}
	msg := parsedURL.Query().Get("error_description")
	// error_hint is sent by ORY Hydra Server only.
	if hint := parsedURL.Query().Get("error_hint"); hint != "" {
		msg = fmt.Sprintf("%s: %s", msg, hint)
	}
	return errors.New(errors.KindOIDCError, msg)
}
