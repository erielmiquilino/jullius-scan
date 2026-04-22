package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// UserAgent is the browser user-agent used by the scraping worker.
// The mobile WebView must use this same UA so that Cloudflare session cookies
// remain valid when replayed by the worker.
const UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// ExecutorResult holds the raw HTML extracted from a SEFAZ page.
type ExecutorResult struct {
	HTML      string
	PageTitle string
	FinalURL  string
}

// BrowserCookie mirrors the JSON shape exchanged with the mobile app.
type BrowserCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires,omitempty"`
	HTTPOnly bool    `json:"http_only"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"same_site,omitempty"`
}

// Executor handles browser automation via chromedp to fetch rendered SEFAZ pages.
type Executor struct {
	timeout time.Duration
}

// NewExecutor creates a new browser executor with the given per-job timeout.
func NewExecutor(timeout time.Duration) *Executor {
	return &Executor{timeout: timeout}
}

// FetchPage navigates to the given fiscal URL, waits for the page to render,
// and returns the outer HTML of the document body.
func (e *Executor) FetchPage(ctx context.Context, fiscalURL string) (*ExecutorResult, error) {
	return e.fetchPage(ctx, fiscalURL, nil, "")
}

// FetchPageWithCookies resumes a scraping session by injecting the provided cookies
// into the browser before navigating to the given URL. userAgent overrides the default
// UserAgent constant — pass the value captured from the mobile WebView so that
// Cloudflare session cookies remain valid for the replayed request.
func (e *Executor) FetchPageWithCookies(ctx context.Context, fiscalURL string, cookies []BrowserCookie, userAgent string) (*ExecutorResult, error) {
	return e.fetchPage(ctx, fiscalURL, cookies, userAgent)
}

// fetchPage is the shared implementation for FetchPage and FetchPageWithCookies.
func (e *Executor) fetchPage(ctx context.Context, fiscalURL string, cookies []BrowserCookie, userAgent string) (*ExecutorResult, error) {
	ua := UserAgent
	if userAgent != "" {
		ua = userAgent
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.UserAgent(ua),
		)...,
	)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(func(s string, i ...interface{}) {
			slog.Debug(fmt.Sprintf("chromedp: "+s, i...))
		}),
	)
	defer browserCancel()

	var bodyHTML string
	var pageTitle string
	var finalURL string

	slog.Info("executor: navigating to fiscal URL", "url", fiscalURL, "resume", len(cookies) > 0)

	var tasks []chromedp.Action

	// If resuming from captcha, inject cookies before navigation.
	if len(cookies) > 0 {
		for _, c := range cookies {
			cookie := c // capture loop variable
			tasks = append(tasks, chromedp.ActionFunc(func(ctx context.Context) error {
				expr := network.SetCookie(cookie.Name, cookie.Value).
					WithDomain(cookie.Domain).
					WithPath(cookie.Path).
					WithHTTPOnly(cookie.HTTPOnly).
					WithSecure(cookie.Secure)
				if cookie.Expires > 0 {
					ts := cdp.TimeSinceEpoch(time.Unix(int64(cookie.Expires), 0))
					expr = expr.WithExpires(&ts)
				}
				return expr.Do(ctx)
			}))
		}
	}

	tasks = append(tasks,
		chromedp.Navigate(fiscalURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.Title(&pageTitle),
		chromedp.Location(&finalURL),
		chromedp.OuterHTML("body", &bodyHTML, chromedp.ByQuery),
	)

	if err := chromedp.Run(browserCtx, tasks...); err != nil {
		return nil, fmt.Errorf("chromedp execution failed: %w", err)
	}

	if strings.TrimSpace(bodyHTML) == "" {
		return nil, fmt.Errorf("extracted HTML body is empty")
	}

	slog.Info("executor: page fetched successfully",
		"url", fiscalURL,
		"final_url", finalURL,
		"title", pageTitle,
		"html_length", len(bodyHTML),
	)

	return &ExecutorResult{
		HTML:      bodyHTML,
		PageTitle: pageTitle,
		FinalURL:  finalURL,
	}, nil
}

// ExtractCurrentState captures the browser's current URL and all cookies.
// Used to persist state when a captcha challenge is detected mid-scrape.
func ExtractCurrentState(ctx context.Context) (currentURL string, cookiesJSON json.RawMessage, err error) {
	var cookies []*network.Cookie
	err = chromedp.Run(ctx,
		chromedp.Location(&currentURL),
		chromedp.ActionFunc(func(ctx context.Context) error {
			c, e := network.GetCookies().Do(ctx)
			cookies = c
			return e
		}),
	)
	if err != nil {
		return "", nil, fmt.Errorf("extract current state: %w", err)
	}

	var out []BrowserCookie
	for _, c := range cookies {
		out = append(out, BrowserCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  float64(c.Expires),
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: c.SameSite.String(),
		})
	}

	data, err := json.Marshal(out)
	if err != nil {
		return "", nil, fmt.Errorf("marshal cookies: %w", err)
	}
	return currentURL, data, nil
}

// TwoPhaseResult is returned by FetchSummaryAndDetail.
type TwoPhaseResult struct {
	// Summary is the result from navigating to the NFC-e summary page. Always set on success.
	Summary *ExecutorResult
	// Detail is the result from navigating to the NFC-e detail page. Nil if captcha was
	// detected on the detail page or if the detail navigation failed.
	Detail *ExecutorResult
	// DetailURL is the URL navigated to for the detail page (may differ from FinalURL if redirected).
	DetailURL string
	// DetailCaptcha is true if a captcha challenge was encountered on the detail page.
	DetailCaptcha bool
	// DetailCookies holds the live browser cookies captured when a detail-page captcha fired,
	// so the worker can persist them and allow the user to resume from that point.
	DetailCookies json.RawMessage
}

// FetchWithDetailPhase navigates to the NFC-e summary URL, extracts the "Ver NFC-e
// detalhada" link from the rendered HTML, then navigates to the detail page within the
// same browser session. Both navigations share cookies, maximising the chance of passing
// Cloudflare challenges on the detail page.
//
// If captcha is detected on the detail page, Detail is nil, DetailCaptcha is true,
// and DetailCookies contains the cookies captured from the live browser.
//
// cookies and userAgent are injected before the summary navigation (used when resuming
// a summary-phase captcha pause with an active session).
//
// If the detail link is not found in the summary HTML (e.g. when cookies already land
// directly on the receipt page), TwoPhaseResult.Detail is nil and DetailURL is empty —
// the caller should persist items without barcode.
func (e *Executor) FetchWithDetailPhase(ctx context.Context, summaryURL string, cookies []BrowserCookie, userAgent string) (*TwoPhaseResult, error) {
	ua := UserAgent
	if userAgent != "" {
		ua = userAgent
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.UserAgent(ua),
		)...,
	)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(func(s string, i ...interface{}) {
			slog.Debug(fmt.Sprintf("chromedp: "+s, i...))
		}),
	)
	defer browserCancel()

	// -- Phase 1: summary page --

	var summaryHTML, summaryTitle, summaryFinalURL string

	var summaryTasks []chromedp.Action
	for _, c := range cookies {
		cookie := c
		summaryTasks = append(summaryTasks, chromedp.ActionFunc(func(ctx context.Context) error {
			expr := network.SetCookie(cookie.Name, cookie.Value).
				WithDomain(cookie.Domain).
				WithPath(cookie.Path).
				WithHTTPOnly(cookie.HTTPOnly).
				WithSecure(cookie.Secure)
			if cookie.Expires > 0 {
				ts := cdp.TimeSinceEpoch(time.Unix(int64(cookie.Expires), 0))
				expr = expr.WithExpires(&ts)
			}
			return expr.Do(ctx)
		}))
	}
	summaryTasks = append(summaryTasks,
		chromedp.Navigate(summaryURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.Title(&summaryTitle),
		chromedp.Location(&summaryFinalURL),
		chromedp.OuterHTML("body", &summaryHTML, chromedp.ByQuery),
	)

	slog.Info("executor: navigating to summary (two-phase)", "url", summaryURL)
	if err := chromedp.Run(browserCtx, summaryTasks...); err != nil {
		return nil, fmt.Errorf("summary navigation failed: %w", err)
	}
	if strings.TrimSpace(summaryHTML) == "" {
		return nil, fmt.Errorf("summary page returned empty body")
	}

	summaryResult := &ExecutorResult{HTML: summaryHTML, PageTitle: summaryTitle, FinalURL: summaryFinalURL}

	// -- Phase 2: extract detail link from summary, then navigate --

	detailURL, err := ExtractDetailLink(summaryHTML, summaryFinalURL)
	if err != nil {
		slog.Info("executor: detail link not found in summary, skipping detail phase", "summary_url", summaryURL, "reason", err)
		return &TwoPhaseResult{Summary: summaryResult}, nil
	}

	slog.Info("executor: navigating to detail page (two-phase)", "url", detailURL)

	var detailHTML, detailTitle, detailFinalURL string
	detailErr := chromedp.Run(browserCtx,
		chromedp.Navigate(detailURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Sleep(2*time.Second),
		chromedp.Title(&detailTitle),
		chromedp.Location(&detailFinalURL),
		chromedp.OuterHTML("body", &detailHTML, chromedp.ByQuery),
	)
	if detailErr != nil {
		// Navigation failure on detail page is non-fatal for the summary result.
		slog.Warn("executor: detail page navigation failed", "url", detailURL, "error", detailErr)
		return &TwoPhaseResult{Summary: summaryResult}, nil
	}

	if strings.TrimSpace(detailHTML) == "" {
		slog.Warn("executor: detail page returned empty body", "url", detailURL)
		return &TwoPhaseResult{Summary: summaryResult}, nil
	}

	detailResult := &ExecutorResult{HTML: detailHTML, PageTitle: detailTitle, FinalURL: detailFinalURL}

	if DetectCaptcha(detailResult) {
		// Captcha on detail page — extract live cookies before closing the browser.
		_, cookiesJSON, cookieErr := ExtractCurrentState(browserCtx)
		if cookieErr != nil {
			slog.Warn("executor: failed to extract detail-phase cookies", "error", cookieErr)
			cookiesJSON = json.RawMessage(`[]`)
		}
		slog.Warn("executor: captcha detected on detail page",
			"detail_url", detailURL,
			"title", detailTitle,
		)
		return &TwoPhaseResult{
			Summary:       summaryResult,
			DetailURL:     detailURL,
			DetailCaptcha: true,
			DetailCookies: cookiesJSON,
		}, nil
	}

	slog.Info("executor: detail page fetched successfully",
		"url", detailURL,
		"final_url", detailFinalURL,
		"html_length", len(detailHTML),
	)
	return &TwoPhaseResult{
		Summary:   summaryResult,
		Detail:    detailResult,
		DetailURL: detailURL,
	}, nil
}

// DetectCaptcha checks the extracted HTML and page title for common SEFAZ
// captcha indicators. Returns true if a captcha or anti-bot block is detected.
func DetectCaptcha(result *ExecutorResult) bool {
	if result == nil {
		return false
	}

	lowerHTML := strings.ToLower(result.HTML)
	lowerTitle := strings.ToLower(result.PageTitle)

	captchaIndicators := []string{
		"captcha",
		"recaptcha",
		"hcaptcha",
		"g-recaptcha",
		"cf-turnstile",
		"challenge-form",
		"challenge-running",
		"bot detection",
		"verificação de segurança",
		"acesso negado",
	}

	for _, indicator := range captchaIndicators {
		if strings.Contains(lowerHTML, indicator) || strings.Contains(lowerTitle, indicator) {
			slog.Warn("captcha detected in page",
				"indicator", indicator,
				"title", result.PageTitle,
			)
			return true
		}
	}

	return false
}
