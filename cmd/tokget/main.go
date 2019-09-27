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
	"os"
	"strings"

	"github.com/i-core/tokget/internal/errors"
	"github.com/i-core/tokget/internal/log"
	"github.com/i-core/tokget/internal/oidc"
)

// version will be filled at compile time.
var version = ""

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
	loginCmd.StringVar(&loginCnf.RedirectURI, "r", "http://localhost:3000", "an OpenID Connect client's redirect uri")
	loginCmd.StringVar(&scopes, "s", "openid,profile,email", "OpenID Connect scopes")
	loginCmd.StringVar(&loginCnf.Username, "u", "", "a user's name")
	loginCmd.StringVar(&loginCnf.Password, "p", "", "a user's password")
	loginCmd.BoolVar(&loginCnf.PasswordStdin, "pwd-stdin", false, "a user's password from stdin")
	loginCmd.StringVar(&loginCnf.UsernameField, "username-field", "input[name=username]", "a CSS selector of the username field on the login form")
	loginCmd.StringVar(&loginCnf.PasswordField, "password-field", "input[name=password]", "a CSS selector of the password field on the login form")
	loginCmd.StringVar(&loginCnf.SubmitButton, "submit-button", "button[type=submit]", "a CSS selector of the submit button on the login form")
	loginCmd.StringVar(&loginCnf.ErrorMessage, "error-message", "p.message", "a CSS selector of an error message on the login form")
	loginCmd.BoolVar(&verboseLogin, "v", false, "verbose mode")

	logoutCnf := &oidc.LogoutConfig{}
	logoutCmd := flag.NewFlagSet("logout", flag.ExitOnError)
	logoutCmd.StringVar(&logoutCnf.Endpoint, "e", "", "an OpenID Connect endpoint")
	logoutCmd.StringVar(&logoutCnf.IDToken, "t", "", "an ID token")
	logoutCmd.BoolVar(&verboseLogout, "v", false, "verbose mode")

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
		case loginCmd.Name():
			loginCmd.Parse(args[1:])

			loginCnf.Scopes = strings.ReplaceAll(scopes, ",", " ")

			ctx := context.Background()
			if verboseLogin {
				ctx = log.WithDebugger(ctx, log.VerboseDebugger)
			}
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

			ctx := context.Background()
			if verboseLogout {
				ctx = log.WithDebugger(ctx, log.VerboseDebugger)
			}
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

const usage = `
usage: tokget [options] <command> [options]

Options:
 --remote-chrome <url>   A remote Google Chrome's url

Commands:
 login   Logs a user in and returns its access token and ID token.
 logout  Logs a user out.
 version Prints version of the tool.
 help    Prints help about the tool.
`
