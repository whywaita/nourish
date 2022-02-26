package nosh

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

var (
	// BaseAPI is url of nosh
	BaseAPI = "https://nosh.jp"
)

// UserInformation is information of login user
type UserInformation struct {
	Cookies []*network.Cookie
	UserID  int
}

// Login is retrieve cookies
func Login(ctx context.Context, email, password string) (*UserInformation, error) {
	var dashboardURL string
	var cookies []*network.Cookie

	if err := chromedp.Run(ctx, chromedp.Tasks{
		chromedp.Navigate(BaseAPI + "/login"),
		chromedp.WaitVisible(`//input[@name="email"]`),
		chromedp.SendKeys(`//input[@name="email"]`, email),
		chromedp.SendKeys(`//input[@name="password"]`, password),
		chromedp.Submit(`//input[@name="password"]`),
		chromedp.Location(&dashboardURL),
		getCookiesAction(&cookies),
	}); err != nil {
		return nil, fmt.Errorf("chromedp.Run(ctx): %w", err)
	}

	userID, err := extractUserID(dashboardURL)
	if err != nil {
		return nil, fmt.Errorf("extractUserID(%s): %w", dashboardURL, err)
	}

	return &UserInformation{
		Cookies: cookies,
		UserID:  userID,
	}, nil
}

// extractUserID retrieve user ID from dashboardURL
func extractUserID(dashboardURL string) (int, error) {
	// dashboardURL e.g. https://nosh.jp/mypage/([0-9]{5])/dashboard
	u, err := url.Parse(dashboardURL)
	if err != nil {
		return -1, fmt.Errorf("url.Parse(%s): %w", dashboardURL, err)
	}

	paths := strings.Split(u.Path, "/")
	if len(paths) != 4 {
		return -1, fmt.Errorf("invalid format of dashboard path (path: %s)", u.Path)
	}
	userid := paths[2]
	id, err := strconv.Atoi(userid)
	if err != nil {
		return -1, fmt.Errorf("strconv.Atoi(%s): %w", userid, err)
	}

	return id, nil
}

func getCookiesAction(cookiesParam *[]*network.Cookie) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		cookies, err := network.GetAllCookies().Do(ctx)
		if err != nil {
			return fmt.Errorf("network.GetAllCookies().Do(ctx): %w", err)
		}
		cookiesParam = &cookies
		return nil
	})
}

// SetCookiesAction set cookies to chromedp
func SetCookiesAction(cookies []*network.Cookie) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		cc := make([]*network.CookieParam, 0, len(cookies))
		for _, c := range cookies {
			cc = append(cc, &network.CookieParam{
				Name:   c.Name,
				Value:  c.Value,
				Domain: c.Domain,
			})
		}
		return network.SetCookies(cc).Do(ctx)
	})
}
