package nosh

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func getAuthorizedOuterHTML(ctx context.Context, cookies []*network.Cookie, htmlURL string) (string, error) {
	var body string
	if err := chromedp.Run(ctx, chromedp.Tasks{
		SetCookiesAction(cookies),
		chromedp.Navigate(htmlURL),
		chromedp.OuterHTML("html", &body, chromedp.ByQuery),
	}); err != nil {
		return "", fmt.Errorf("chromedp.Run(ctx): %w", err)
	}

	return body, nil
}
