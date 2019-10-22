/*
Copyright (c) JSC iCore.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package main // import "github.com/i-core/tokget/cmd/tokget"

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/i-core/rlog"
	"github.com/i-core/routegroup"
	"github.com/i-core/tokget/internal/errors"
	"github.com/i-core/tokget/internal/log"
	"github.com/i-core/tokget/internal/oidc"
	"github.com/i-core/tokget/internal/stat"
	"github.com/i-core/tokget/internal/web"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

// version will be filled at compile time.
var version = ""

// serveConfig is a server's configuration.
type serveConfig struct {
	Listen  string `default:":8080" desc:"a host and port to listen on (<host>:<port>)"`
	Verbose bool   `default:"false" desc:"a development mode"`
}

// The default values of the login operation's parameters.
const (
	defaultRedirectURI   = "http://localhost:3000"
	defaultScopes        = "openid,profile,email"
	defaultUsernameField = "input[name=username]"
	defaultPasswordField = "input[name=password]"
	defaultSubmitButton  = "button[type=submit]"
	defaultErrorMessage  = "p.message"
)

func main() {
	var (
		verboseLogin  bool
		verboseLogout bool
		scopes        string
	)

	loginCnf := &oidc.LoginConfig{}
	loginCmd := flag.NewFlagSet("login", flag.ExitOnError)
	loginCmd.StringVar(&loginCnf.Endpoint, "e", "", "an OpenID Connect endpoint")
	loginCmd.StringVar(&loginCnf.ClientID, "c", "", "an OpenID Connect client ID")
	loginCmd.StringVar(&loginCnf.RedirectURI, "r", defaultRedirectURI, "an OpenID Connect client's redirect uri")
	loginCmd.StringVar(&scopes, "s", defaultScopes, "OpenID Connect scopes")
	loginCmd.StringVar(&loginCnf.Username, "u", "", "a user's name")
	loginCmd.StringVar(&loginCnf.Password, "p", "", "a user's password")
	loginCmd.StringVar(&loginCnf.UsernameField, "username-field", defaultUsernameField, "a CSS selector of the username field on the login form")
	loginCmd.StringVar(&loginCnf.PasswordField, "password-field", defaultPasswordField, "a CSS selector of the password field on the login form")
	loginCmd.StringVar(&loginCnf.SubmitButton, "submit-button", defaultSubmitButton, "a CSS selector of the submit button on the login form")
	loginCmd.StringVar(&loginCnf.ErrorMessage, "error-message", defaultErrorMessage, "a CSS selector of an error message on the login form")
	loginCmd.BoolVar(&verboseLogin, "v", false, "verbose mode")

	logoutCnf := &oidc.LogoutConfig{}
	logoutCmd := flag.NewFlagSet("logout", flag.ExitOnError)
	logoutCmd.StringVar(&logoutCnf.Endpoint, "e", "", "an OpenID Connect endpoint")
	logoutCmd.StringVar(&logoutCnf.IDToken, "t", "", "an ID token")
	logoutCmd.BoolVar(&verboseLogout, "v", false, "verbose mode")

	serveCnf := &serveConfig{}
	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	serveCmd.StringVar(&serveCnf.Listen, "l", "", "a host and port to listen on (<host>:<port>)")
	serveCmd.BoolVar(&serveCnf.Verbose, "v", false, "verbose mode")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
	}

	if len(os.Args) == 1 {
		flag.Usage()
		os.Exit(1)
	}

	var chromeURL string
	args := os.Args[1:]
	for len(args) > 0 {
		switch arg := args[0]; arg {
		case "version":
			fmt.Fprintln(flag.CommandLine.Output(), version)
			os.Exit(0)
		case "help", "-h", "--help":
			flag.Usage()
			os.Exit(1)
		case "--remote-chrome":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "Error: option --remote-chrome has no value")
				os.Exit(1)
			}
			chromeURL = args[1]
			args = args[2:]
		case serveCmd.Name():
			if err := envconfig.Process("tokget", serveCnf); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid configuration: %s\n", err)
				os.Exit(1)
			}

			serveCmd.Parse(args[1:])

			logFunc := zap.NewProduction
			if serveCnf.Verbose {
				logFunc = zap.NewDevelopment
			}
			log, err := logFunc()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create logger: %s\n", err)
				os.Exit(1)
			}

			router := routegroup.NewRouter(rlog.NewMiddleware(log))
			loginFn := func(ctx context.Context, cnf *oidc.LoginConfig) (*oidc.LoginData, error) {
				return oidc.Login(ctx, chromeURL, cnf)
			}
			logoutFn := func(ctx context.Context, cnf *oidc.LogoutConfig) error {
				return oidc.Logout(ctx, chromeURL, cnf)
			}
			defaults := web.Defaults{
				RedirectURI:   defaultRedirectURI,
				Scopes:        strings.ReplaceAll(defaultScopes, ",", " "),
				UsernameField: defaultUsernameField,
				PasswordField: defaultPasswordField,
				SubmitButton:  defaultSubmitButton,
				ErrorMessage:  defaultErrorMessage,
			}
			router.AddRoutes(web.NewHandler(defaults, loginFn, logoutFn, rlog.FromContext), "")
			router.AddRoutes(stat.NewHandler(version), "/stat")

			log = log.Named("main")
			log.Info("Tokget started", zap.Any("config", serveCnf), zap.String("version", version))
			log.Fatal("Tokget finished", zap.Error(http.ListenAndServe(serveCnf.Listen, router)))
			os.Exit(0)
		case loginCmd.Name():
			loginCmd.Parse(args[1:])

			loginCnf.Scopes = strings.ReplaceAll(scopes, ",", " ")

			ctx := withInterrupt(context.Background())
			ctx = log.WithLogger(ctx, verboseLogin)

			v, err := oidc.Login(ctx, chromeURL, loginCnf)
			if err != nil {
				if errors.Cause(err) != context.Canceled {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				}
				os.Exit(1)
			}
			b, err := json.Marshal(v)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: encode user data to JSON: %s\n", err)
				os.Exit(1)
			}
			fmt.Fprintln(flag.CommandLine.Output(), string(b))
			os.Exit(0)
		case logoutCmd.Name():
			logoutCmd.Parse(args[1:])

			ctx := withInterrupt(context.Background())
			ctx = log.WithLogger(ctx, verboseLogout)

			err := oidc.Logout(ctx, chromeURL, logoutCnf)
			if err != nil {
				if errors.Cause(err) != context.Canceled {
					fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				}
				os.Exit(1)
			}
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "%q is not valid command.\n", arg)
			os.Exit(1)
		}
	}

	flag.Usage()
	os.Exit(1)
}

// withInterrupt returns a new context that will be canceled when the program receives an interruption signal (SIGINT).
func withInterrupt(parent context.Context) context.Context {
	ctx, cancel := context.WithCancel(parent)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}

const usage = `
usage: tokget [options] <command> [options]

Options:
 --remote-chrome <url>   A remote Google Chrome's URL

Commands:
 login   Logs a user in and returns its access token and ID token.
 logout  Logs a user out.
 serve   Starts a web server that allows logging in and out.
 version Prints version of the tool.
 help    Prints help about the tool.
`
