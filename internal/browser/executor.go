// Copyright 2020 the Exposure Notifications Verification Server authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package browser

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
)

// defaultOptions are the default Chrome options.
var defaultOptions = [...]chromedp.ExecAllocatorOption{
	chromedp.NoFirstRun,
	chromedp.NoDefaultBrowserCheck,
	chromedp.WindowSize(1920, 1080),

	// After Puppeteer's default behavior.
	chromedp.Flag("disable-background-networking", true),
	chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
	chromedp.Flag("disable-background-timer-throttling", true),
	chromedp.Flag("disable-backgrounding-occluded-windows", true),
	chromedp.Flag("disable-breakpad", true),
	chromedp.Flag("disable-client-side-phishing-detection", true),
	chromedp.Flag("disable-default-apps", true),
	chromedp.Flag("disable-dev-shm-usage", true),
	chromedp.Flag("disable-extensions", true),
	chromedp.Flag("disable-features", "site-per-process,TranslateUI,BlinkGenPropertyTrees"),
	chromedp.Flag("disable-hang-monitor", true),
	chromedp.Flag("disable-ipc-flooding-protection", true),
	chromedp.Flag("disable-popup-blocking", true),
	chromedp.Flag("disable-prompt-on-repost", true),
	chromedp.Flag("disable-renderer-backgrounding", true),
	chromedp.Flag("disable-sync", true),
	chromedp.Flag("force-color-profile", "srgb"),
	chromedp.Flag("metrics-recording-only", true),
	chromedp.Flag("safebrowsing-disable-auto-update", true),
	chromedp.Flag("enable-automation", true),
	chromedp.Flag("password-store", "basic"),
	chromedp.Flag("use-mock-keychain", true),
}

// New creates a new headless browser context. See NewFromOptions for usage.
func New(tb testing.TB) context.Context {
	tb.Helper()
	opts := defaultOptions[:]
	opts = append(opts, chromedp.Headless)
	return NewFromOptions(tb, opts)
}

// NewHeadful creates a new browser context so you can actually watch the test.
// This is for local debugging and will fail on CI where a browser isn't
// actually available.
func NewHeadful(tb testing.TB) context.Context {
	tb.Helper()
	return NewFromOptions(tb, defaultOptions[:])
}

// NewFromOptions creates a new browser instance. All future calls to `Run` must
// use the context returned by this function!
//
// If this function returns successfully, a browser is running and ready to be
// used. It's recommended that you wrap the returned context in a timeout.
func NewFromOptions(tb testing.TB, opts []chromedp.ExecAllocatorOption) context.Context {
	tb.Helper()

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	tb.Cleanup(cancel)

	taskCtx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(tb.Logf))
	tb.Cleanup(cancel)

	// Start browser
	if err := chromedp.Run(taskCtx); err != nil {
		tb.Fatal(err)
	}

	return taskCtx
}

// Screenshot captures a screenshot of the browser page in its current state.
// This is useful for debugging a test failure. The dst will contain the
// screenshot bytes in PNG format when the runner finishes.
func Screenshot(dst *[]byte) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, contentSize, _, _, _, err := page.GetLayoutMetrics().Do(ctx)
		if err != nil {
			return err
		}

		width, height := int64(math.Ceil(contentSize.Width)), int64(math.Ceil(contentSize.Height))

		err = emulation.
			SetDeviceMetricsOverride(width, height, 1, false).
			WithScreenOrientation(&emulation.ScreenOrientation{
				Type:  emulation.OrientationTypePortraitPrimary,
				Angle: 0,
			}).
			Do(ctx)
		if err != nil {
			return err
		}

		// capture screenshot
		*dst, err = page.CaptureScreenshot().
			WithQuality(100).
			WithClip(&page.Viewport{
				X:      contentSize.X,
				Y:      contentSize.Y,
				Width:  contentSize.Width,
				Height: contentSize.Height,
				Scale:  2,
			}).Do(ctx)
		if err != nil {
			return err
		}
		return nil
	})
}

// SetCookie sets a cookie with the provided parameters. This can be used to
// bypass login and force a specific user be logged in during the test.
func SetCookie(c *http.Cookie) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		exp := cdp.TimeSinceEpoch(time.Now().Add(24 * time.Hour))

		if err := network.
			SetCookie(c.Name, c.Value).
			WithPath(c.Path).
			WithDomain(c.Domain).
			WithExpires(&exp).
			WithSecure(c.Secure).
			WithHTTPOnly(c.HttpOnly).
			Do(ctx); err != nil {
			return fmt.Errorf("failed to set cookie: %w", err)
		}
		return nil
	})
}

// Cookies sets the current list of cookies into the provided destination.
func Cookies(dst *[]*http.Cookie) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		networkCookies, err := storage.GetCookies().Do(ctx)
		if err != nil {
			return err
		}

		httpCookies := make([]*http.Cookie, len(networkCookies))
		for i, c := range networkCookies {
			httpCookies[i] = &http.Cookie{
				Name:     c.Name,
				Value:    c.Value,
				Path:     c.Path,
				Domain:   c.Domain,
				Expires:  time.Unix(int64(c.Expires), 0),
				Secure:   c.Secure,
				HttpOnly: c.HTTPOnly,
			}
		}
		*dst = httpCookies

		return nil
	})
}
