package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// ExecutorResult holds the raw HTML extracted from a SEFAZ page.
type ExecutorResult struct {
	HTML      string
	PageTitle string
	FinalURL  string
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
//
// The caller is responsible for passing a context with an appropriate deadline.
// This method creates its own chromedp allocator and browser context, ensuring
// cleanup on return regardless of success or failure.
func (e *Executor) FetchPage(ctx context.Context, fiscalURL string) (*ExecutorResult, error) {
	// Create a headless Chrome allocator with sandbox disabled for container environments.
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("disable-dev-shm-usage", true),
		)...,
	)
	defer allocCancel()

	// Create a browser context from the allocator.
	browserCtx, browserCancel := chromedp.NewContext(allocCtx,
		chromedp.WithLogf(func(s string, i ...interface{}) {
			slog.Debug(fmt.Sprintf("chromedp: "+s, i...))
		}),
	)
	defer browserCancel()

	var bodyHTML string
	var pageTitle string
	var finalURL string

	slog.Info("executor: navigating to fiscal URL", "url", fiscalURL)

	// Run the browser automation tasks.
	err := chromedp.Run(browserCtx,
		// Navigate to the fiscal URL.
		chromedp.Navigate(fiscalURL),

		// Wait for the page body to be visible — SEFAZ pages render via JS.
		chromedp.WaitVisible("body", chromedp.ByQuery),

		// Give JS time to populate dynamic content.
		chromedp.Sleep(2*time.Second),

		// Extract the page title for diagnostics.
		chromedp.Title(&pageTitle),

		// Get the current URL (may have redirected).
		chromedp.Location(&finalURL),

		// Extract the full body HTML.
		chromedp.OuterHTML("body", &bodyHTML, chromedp.ByQuery),
	)
	if err != nil {
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
