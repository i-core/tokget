/*
Copyright (c) JSC iCore.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package errors

import (
	"fmt"
)

// Kind describes an error kind.
type Kind string

const (
	// KindOther is a kind for an untyped errors.
	// This kind is used by default when a error kind is not specified.
	KindOther Kind = ""
	// KindEndpointMissed is a kind of an error that happens when OpenID Connect endpoint is not specified.
	KindEndpointMissed Kind = "endpoint_is_missed"
	// KindEndpointInvalid is a kind of an error that happens when OpenID Connect endpoint is invalid.
	KindEndpointInvalid Kind = "endpoint_is_invalid"
	// KindClientIDMissed is a kind of an error that happens when OpenID Connect client ID is not specified.
	KindClientIDMissed Kind = "client_id_is_missed"
	// KindRedirectURIMissed is a kind of an error that happens when OpenID Connect client's redirect URI is not specified.
	KindRedirectURIMissed Kind = "redirect_uri_is_missed"
	// KindScopesMissed is a kind of an error that happens when OpenID Connect scopes are not specified.
	KindScopesMissed Kind = "scopes_are_missed"
	// KindIDTokenMissed is a kind of an error that happens when ID token is not specified.
	KindIDTokenMissed Kind = "id_token_is_missed"
	// KindUsernameMissed is a kind of an error that happens when a username is not specified.
	KindUsernameMissed Kind = "username_is_missed"
	// KindUsernameFieldMissed is a kind of an error that happens when a username field's selector is not specified.
	KindUsernameFieldMissed Kind = "username_field_selector_is_missed"
	// KindUsernameFieldInvalid is a kind of an error that happens when a username field's selector matches to an invalid form element.
	KindUsernameFieldInvalid Kind = "username_field_selector_is_invalid"
	// KindPasswordFieldMissed is a kind of an error that happens when a password field's selector is not specified.
	KindPasswordFieldMissed Kind = "password_field_selector_is_missed"
	// KindPasswordFieldInvalid is a kind of an error that happens when a password field's selector matches to an invalid form element.
	KindPasswordFieldInvalid Kind = "password_field_selector_is_invalid"
	// KindSubmitButtonMissed is a kind of an error that happens when a submit button's selector is not specified.
	KindSubmitButtonMissed Kind = "submit_button_selector_is_missed"
	// KindSubmitButtonInvalid is a kind of an error that happens when a submit button's selector matches to an invalid form element.
	KindSubmitButtonInvalid Kind = "submit_button_selector_is_invalid"
	// KindErrorMessageMissed is a kind of an error that happens when an error message's selector is not specified.
	KindErrorMessageMissed Kind = "error_message_selector_is_missed"
	// KindOIDCError is a kind of an error that is an OpenID Connect errors.
	KindOIDCError Kind = "openid_connect_error"
	// KindLoginError is a kind of an error that happens when authentication failed, for example, when username or password are invalid.
	KindLoginError Kind = "login_error"
	// KindTimeout is a kind of an error that happens when page loading exceeds a timeout.
	KindTimeout Kind = "timeout"
)

func (k Kind) String() string {
	if k == KindOther {
		return "error"
	}
	return string(k)
}

// Error is an error's wrapper allows to add some context to an underlying error.
// and returns a pretty formatted error's message.
type Error struct {
	Kind  Kind
	cause error
	msg   string
}

func (e *Error) Error() string {
	if e.cause == nil {
		return e.msg
	}
	return fmt.Sprintf("%s: %s", e.msg, e.cause)
}

// New returns a new Error constructed from its arguments.
// The type of each argument determines its meaning.
// If more than one argument of a given type is presented, only the last one is recorded.
//
// The types are:
// 	errKind
//		The class of error, such as IO error.
// 	errParam
//		A business object's location with which the error happened.
// 	string
// 		Treated as an error message.
//
// Arguments having another type is treated as the message's parameters.
// We recommend to place them at the end of argument's list to avoid confusing.
func New(args ...interface{}) *Error {
	var (
		err    = &Error{}
		msg    string
		params []interface{}
	)
	for _, arg := range args {
		switch arg := arg.(type) {
		case Kind:
			err.Kind = arg
		case error:
			err.cause = arg
		case string:
			if msg == "" {
				msg = arg
			} else {
				params = append(params, arg)
			}
		default:
			params = append(params, arg)
		}
	}
	err.msg = fmt.Sprintf(msg, params...)
	return err
}

// Wrap returns a new error that wraps the specified error with a message.
func Wrap(err error, msg string, params ...interface{}) *Error {
	args := []interface{}{err, msg}
	args = append(args, params...)
	return New(args...)
}

// Cause returns a cause of Error if the cause exists.
// The function returns the original error if it is not an instance of Error.
func Cause(err error) error {
	v, ok := err.(*Error)
	if !ok {
		return err
	}
	return Cause(v.cause)
}

// Match returns true when specified errors are similar, and false when they are not.
// The errors are considered similar when they have the type "Error",
// and values of fields "kind" and "param" of the "want" error equal to values of the same fields of the "got" error.
func Match(got, want error) bool {
	w, ok := want.(*Error)
	if !ok {
		return false
	}
	g, ok := got.(*Error)
	if !ok {
		return false
	}
	return w.Kind == g.Kind
}
