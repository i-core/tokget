/*
Copyright (c) JSC iCore.
This source code is licensed under the MIT license found in the
LICENSE file in the root directory of this source tree.
*/

package chrome

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/i-core/tokget/internal/errors"
	"github.com/i-core/tokget/internal/log"
)

// Domain is a Chrome DevTools Protocol domain.
type Domain string

const (
	domainPage Domain = "page"
	// DomainNetwork is the domain "Network". See https://chromedevtools.github.io/devtools-protocol/tot/Network.
	DomainNetwork Domain = "network"
	// DomainRuntime is the domain "Runtime". See https://chromedevtools.github.io/devtools-protocol/tot/Runtime.
	DomainRuntime Domain = "runtime"
)

// ConnectWithContext establishes a connection with a Chrome process by Chrome DevTools Protocol,
// and activates requested domains.
//
// If chromeURL is empty the function starts a new Chrome process by calling command "google-chrome" (must be in $PATH).
// If chromeURL is defined the function connects with a remote Chrome process that is accessible on this URL.
//
// The function returns a context and cancelation function.
// You should use the context to execute a command in the connected Chrome process.
// The cancelation function closes the established connection.
// In the case when the connection established with a new Chrome process
// the function finishes the Chrome process.
func ConnectWithContext(parent context.Context, chromeURL string, domains ...Domain) (context.Context, context.CancelFunc, error) {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	//
	// Establish a connection with a Chrome process.
	//
	logger := log.LoggerFromContext(parent).Sugar()
	if chromeURL == "" {
		logger.Debug("Start a new chrome process")
		ctx, cancel = chromedp.NewContext(parent)
	} else {
		logger.Debugf("Connect to a chrome process at %q\n", chromeURL)
		var err error
		if ctx, cancel, err = connectToRemoteChrome(parent, chromeURL); err != nil {
			return nil, nil, errors.Wrap(err, "connect to remote Chrome process")
		}
	}

	// Prepare the established connection for the program.
	// Put the code to a separate function to simplify canceling the connection in error cases.
	prepareConn := func() error {
		//
		// Check Chrome version. The program depends on event's order of Chrome 70 or higher.
		//
		var cdpVersion, product string
		versionAction := chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cdpVersion, product, _, _, _, err = browser.GetVersion().Do(ctx)
			return err
		})
		if err := chromedp.Run(ctx, versionAction); err != nil {
			return errors.Wrap(err, "get Chrome version")
		}
		logger.Debugf("Chrome info:\n\tprotocolVersion: %s\n\tproduct:         %s\n", cdpVersion, product)
		major, err := majorVersion(product)
		if err != nil {
			return errors.New("invalid Chrome version %q", product)
		}
		if major < 70 {
			return errors.New("unsupported Chrome version %q", product)
		}

		//
		// Activate requested Chrome DevTools Protocol domains.
		//
		domains = append([]Domain{domainPage}, domains...)
		var acts []chromedp.Action
		for _, dm := range domains {
			switch dm {
			case domainPage:
				acts = append(acts, page.Enable())
			case DomainNetwork:
				acts = append(acts, network.Enable())
			case DomainRuntime:
				acts = append(acts, runtime.Enable())
			}
		}
		if err := chromedp.Run(ctx, acts...); err != nil {
			return errors.Wrap(err, "activate CDP domains")
		}

		return nil
	}
	if err := prepareConn(); err != nil {
		cancel()
		return nil, nil, err
	}

	return ctx, cancel, nil
}

// connectToRemoteChrome connect to a remote Chrome process via Chrome DevTool Protocol.
func connectToRemoteChrome(parent context.Context, chromeURL string) (context.Context, context.CancelFunc, error) {
	// Chrome provides an URL for a Chrome DevTool Protocol's connection in a configuration
	// that is served on "<chromeURL>/json". Build a request for a Chrome configuration.
	type chromeConfig struct {
		DebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	u, err := url.Parse(chromeURL)
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid chrome url")
	}
	ref, err := url.Parse("/json")
	if err != nil {
		panic(fmt.Sprintf("build chrome config url: %s", err))
	}
	u = u.ResolveReference(ref)
	r, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "create chrome config request")
	}

	// Load a Chrome configuration.
	reqCtx, cancelReqCtx := context.WithTimeout(parent, 3*time.Second)
	defer cancelReqCtx()
	resp, err := http.DefaultClient.Do(r.WithContext(reqCtx))
	if err != nil {
		return nil, nil, errors.Wrap(err, "load chrome config")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil, errors.New("load chrome config: status code %d", resp.StatusCode)
	}

	// Get an URL for Chrome DevTool Protocol's connection from the Chrome configuration that is loaded above.
	var cnfs []*chromeConfig
	var b []byte
	if b, err = ioutil.ReadAll(resp.Body); err != nil {
		return nil, nil, errors.Wrap(err, "read chrome config")
	}
	if err = json.Unmarshal(b, &cnfs); err != nil {
		return nil, nil, errors.Wrap(err, "parse chrome config")
	}

	if len(cnfs) == 0 {
		if len(b) == 0 {
			return nil, nil, errors.New("unexpected empty chrome config")
		}
		return nil, nil, errors.New("unexpected chrome config:\n%s", string(b))
	}
	logger := log.LoggerFromContext(parent).Sugar()
	logger.Debugf("Remote Chrome Config:\n%s\n", string(b))
	debuggerURL := cnfs[0].DebuggerURL

	// Connect to a remote Chrome process.
	ctx, cancelRemoteAllocator := chromedp.NewRemoteAllocator(parent, debuggerURL)
	ctx, cancelChrome := chromedp.NewContext(ctx)
	return ctx, func() {
		cancelChrome()
		cancelRemoteAllocator()
	}, nil
}

// majorVersion extracts the major version from a Chrome's product name.
//
// Example: HeadlessChrome/70.0.3538.16 -> 70.
func majorVersion(product string) (int, error) {
	productRE := regexp.MustCompile(`^(.+)/(.+)$`)
	res := productRE.FindAllStringSubmatch(product, -1)
	if len(res) != 1 {
		return 0, fmt.Errorf("invalid product %q", product)
	}
	verParts := strings.Split(res[0][2], ".")
	major, err := strconv.Atoi(verParts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid product version %q", product)
	}
	return major, nil
}

// NavHistory provides an access to page's navigation requests.
//
// NavHistory doesn't use the browser history mechanism because of the browser doesn't store
// a navigation request in the history when a request failed with a network error,
// for example, net::ERR_CONNECTION_REFUSED.
type NavHistory struct {
	entries []string
}

// NewNavHistory creates a new NavHistory and listens a Chrome process for navigation requests
// to fill the created NavHistory.
func NewNavHistory(ctx context.Context) (*NavHistory, error) {
	navHistory := &NavHistory{}

	rpattern := &network.RequestPattern{URLPattern: "*", ResourceType: "Document"}
	if err := chromedp.Run(ctx, network.SetRequestInterception([]*network.RequestPattern{rpattern})); err != nil {
		return nil, errors.Wrap(err, "initialize navigation history")
	}

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		v, ok := ev.(*network.EventRequestIntercepted)
		if !ok {
			return
		}
		if v.IsNavigationRequest {
			rurl, err := url.Parse(v.Request.URL)
			if err != nil {
				panic(err)
			}
			if v.Request.URLFragment != "" {
				rurl.Fragment = v.Request.URLFragment[1:]
			}
			navHistory.entries = append(navHistory.entries, rurl.String())
			logger := log.LoggerFromContext(ctx).Sugar()
			logger.Debugf("request %s %s\n", v.Request.Method, rurl.String())
		}
		go func(interceptionID network.InterceptionID) {
			if err := chromedp.Run(ctx, network.ContinueInterceptedRequest(interceptionID)); err != nil {
				panic(fmt.Sprintf("continue request: %s", err))
			}
		}(v.InterceptionID)
	})

	return navHistory, nil
}

// Last returns the last navigation request.
//
// The last navigation request can be different from the current location of the page (see chromedp.Location()),
// for example, in the case of a redirect, or unavailable resource.
//
// | example              | the last navigation request | the current location
// |----------------------|-----------------------------|-----------------------------
// | without redirect     | http://ac.me                | http://ac.me
// | with redirect        | http://ac.me/foo            | http://ac.me/bar
// | unavailable resource | http://ac.me/unavailable    | chrome-error://chromewebdata
func (h *NavHistory) Last() string {
	if len(h.entries) == 0 {
		return ""
	}
	return h.entries[len(h.entries)-1]
}

// Navigate navigates the current page of a Chrome process to an URL and waits for page loading finished.
//
// The function differs from chromedp.Navigate() only that it waits for page loading finished.
//
// See the function NewPageLoadingWaiter for details of waiting.
func Navigate(ctx context.Context, pageURL string) error {
	wait := PageLoadWaiterFunc(ctx, true, 0)
	if err := chromedp.Run(ctx, chromedp.Navigate(pageURL)); err != nil {
		return err
	}
	return wait()
}

// PageLoadWaiterFunc returns a waiting function that waits for finishing of loading the current page.
//
// The waiting function considers that loading is finished when events *network.EventLoadingFinished
// or *network.EventLoadingFailed is fired.
//
// *network.EventLoadingFinished indicates successful page' loading.
// In this case the waiting function returns nil.
//
// *network.EventLoadingFailed indicates a loading page's error.
// In this case the waiting function returns an error. When strict mode is used
// *network.EventLoadingFailed event does not generate an error, and the waiting function returns nil.
//
// The waiting function return errors.Error with the kind errors.KindTimeout when a timeout exceeded (if timeout more than 0),
// and context.Canceled when a context canceled.
func PageLoadWaiterFunc(ctx context.Context, strict bool, timeout time.Duration) func() error {
	var (
		// chromedp does not provide the way to stop listening, so do it manually.
		stoped bool
		ch     = make(chan string, 1)
	)
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if stoped {
			return
		}
		switch ev.(type) {
		case *page.EventLoadEventFired:
			stoped = true
			go func() { ch <- "" }()
		}
	})

	return func() error {
		timeoutChan := make(chan bool, 1)
		if timeout > 0 {
			go func(v time.Duration) {
				<-time.After(v)
				timeoutChan <- true
			}(timeout)
		}

		select {
		case <-ch:
			return nil
		case <-timeoutChan:
			return errors.New(errors.KindTimeout, "timeout")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// HasElement returns true when the current page of a Chrome process contains an element that matches a selector.
func HasElement(ctx context.Context, sel string) (bool, error) {
	var (
		has bool
		act = chromedp.ActionFunc(func(ctx context.Context) error {
			hasElemExpr := fmt.Sprintf("document.querySelector(%q) != null", sel)
			return chromedp.Evaluate(hasElemExpr, &has).Do(ctx)
		})
	)
	if err := chromedp.Run(ctx, act); err != nil {
		return false, err
	}
	if !has {
		return false, nil
	}
	return true, nil
}

// Text returns text of an element that mathes a selector.
//
// The function differs from chromedp.Text() only that it does not blocks the program
// when a page does not contain an element that matches a selector.
func Text(ctx context.Context, sel string) (string, error) {
	has, err := HasElement(ctx, sel)
	if err != nil {
		return "", err
	}
	if !has {
		// The current page does not contain an element that matches a selector.
		return "", nil
	}
	var text string
	if err = chromedp.Run(ctx, chromedp.Text(sel, &text)); err != nil {
		return "", err
	}
	return text, nil
}
