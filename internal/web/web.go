/*
Copyright (c) JSC iCore.

This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/i-core/tokget/internal/errors"
	"github.com/i-core/tokget/internal/oidc"
	"go.uber.org/zap"
)

const errMsgInvalidBody = "invalid body"

// LogFn is a function that creates a logging function for HTTP request.
type LogFn func(ctx context.Context) *zap.Logger

// Defaults is the default parameter of a login config.
type Defaults struct {
	RedirectURI   string
	Scopes        string
	UsernameField string
	PasswordField string
	SubmitButton  string
	ErrorMessage  string
}

// Handler provides HTTP handlers for login and logout operations.
type Handler struct {
	defaults Defaults
	loginFn  LoginFn
	logoutFn LogoutFn
	logFn    LogFn
}

// NewHandler creates a new Handler.
func NewHandler(defaults Defaults, loginFn LoginFn, logoutFn LogoutFn, logFn LogFn) *Handler {
	return &Handler{defaults: defaults, loginFn: loginFn, logoutFn: logoutFn, logFn: logFn}
}

// AddRoutes registers all required routes for login and logout operations.
func (h *Handler) AddRoutes(apply func(m, p string, h http.Handler, mws ...func(http.Handler) http.Handler)) {
	apply(http.MethodPost, "/login", newLoginHandler(h.defaults, h.loginFn, h.logFn))
	apply(http.MethodPost, "/logout", newLogoutHandler(h.logoutFn, h.logFn))
}

// LoginFn is an interface to execute the login operation.
type LoginFn func(ctx context.Context, cnf *oidc.LoginConfig) (*oidc.LoginData, error)

func newLoginHandler(defaults Defaults, loginFn LoginFn, logFn LogFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logFn(r.Context()).Sugar()

		loginCnf := &oidc.LoginConfig{
			RedirectURI:   defaults.RedirectURI,
			Scopes:        defaults.Scopes,
			UsernameField: defaults.UsernameField,
			PasswordField: defaults.PasswordField,
			SubmitButton:  defaults.SubmitButton,
			ErrorMessage:  defaults.ErrorMessage,
		}
		if r.Body != http.NoBody {
			if err := json.NewDecoder(r.Body).Decode(loginCnf); err != nil {
				httpError(w, http.StatusBadRequest, errMsgInvalidBody)
				log.Debug("A payload is invalid", zap.Error(err))
				return
			}
		}

		v, err := loginFn(r.Context(), loginCnf)
		if err != nil {
			if isValidationError(err) {
				httpError(w, http.StatusBadRequest, err.Error())
				log.Debug("A payload is invalid", zap.Error(err))
				return
			}
			httpError(w, http.StatusInternalServerError, "")
			log.Debug("A payload is invalid", zap.Error(err))
			return
		}

		response(w, http.StatusOK, v)
	}
}

// LogoutFn is an interface to execute the logout operation.
type LogoutFn func(ctx context.Context, cnf *oidc.LogoutConfig) error

func newLogoutHandler(logoutFn LogoutFn, logFn LogFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log := logFn(r.Context()).Sugar()

		logoutCnf := &oidc.LogoutConfig{}
		if r.Body != http.NoBody {
			if err := json.NewDecoder(r.Body).Decode(logoutCnf); err != nil {
				httpError(w, http.StatusBadRequest, errMsgInvalidBody)
				log.Debug("A payload is invalid", zap.Error(err))
				return
			}
		}

		err := logoutFn(r.Context(), logoutCnf)
		if err != nil {
			if isValidationError(err) {
				httpError(w, http.StatusBadRequest, err.Error())
				log.Debug("A payload is invalid", zap.Error(err))
				return
			}
			httpError(w, http.StatusInternalServerError, "")
			log.Debug("A payload is invalid", zap.Error(err))
			return
		}

		response(w, http.StatusOK, nil)
	}
}

func isValidationError(err error) bool {
	v, ok := err.(*errors.Error)
	if !ok {
		return false
	}
	validationErrors := []errors.Kind{
		errors.KindEndpointMissed,
		errors.KindClientIDMissed,
		errors.KindRedirectURIMissed,
		errors.KindScopesMissed,
		errors.KindUsernameMissed,
		errors.KindUsernameFieldMissed,
		errors.KindPasswordFieldMissed,
		errors.KindSubmitButtonMissed,
		errors.KindErrorMessageMissed,
		errors.KindEndpointInvalid,
	}
	for _, kind := range validationErrors {
		if kind == v.Kind {
			return true
		}
	}
	return false
}

// ErrorData describes an error's format that all HTTP handlers must use to sends errors to a client.
type ErrorData struct {
	Message string `json:"message"`
}

// httpError writes an error to a request in a standard form.
func httpError(w http.ResponseWriter, code int, message string, params ...interface{}) {
	var msg string
	if code >= 500 || message == "" {
		msg = http.StatusText(code)
	}
	msg = fmt.Sprintf(msg, params...)

	response(w, code, ErrorData{Message: msg})
}

// response writes data to a request in a standard form.
func response(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if data != nil {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(code)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			panic(err)
		}
	}
}
