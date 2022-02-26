package nosh

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"

	"github.com/chromedp/cdproto/network"
)

func getMenuURL(userID, scheduleID int) string {
	return fmt.Sprintf("%s/mypage/%d/%d/menu", BaseAPI, userID, scheduleID)
}

// Menu is menu for nosh
type Menu struct {
	ID        int
	Name      string
	Nutrition Nutrition
	ImageURL  *url.URL
	Count     int
}

// Nutrition is 栄養
type Nutrition struct {
	// Sugar is 糖質
	Sugar float64
	// Salinity is 塩分
	Salinity float64
	// Calorie is カロリー
	Calorie float64
	// Protein is たんぱく質
	Protein float64
	// Fiber is 食物遷移
	Fiber float64
	// Lipid is 脂質
	Lipid float64
}

func (m Menu) detailMenu() string {
	p := path.Join("menu", "detail", strconv.Itoa(m.ID))
	return fmt.Sprintf("%s/%s", BaseAPI, p)
}

// PrettyString to string
func (m Menu) PrettyString() string {
	return fmt.Sprintf("%s %d食 %s", m.Name, m.Count, m.detailMenu())
}

// GetMenuByScheduleID get menus in scheduleID
func GetMenuByScheduleID(ctx context.Context, cookies []*network.Cookie, userID, scheduleID int) ([]Menu, error) {
	var menus []Menu

	if err := chromedp.Run(ctx, chromedp.Tasks{
		SetCookiesAction(cookies),
		chromedp.Navigate(getMenuURL(userID, scheduleID)),
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return fmt.Errorf("dom.GetDocument().Do(ctx): %w", err)
			}
			h, err := dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			if err != nil {
				return fmt.Errorf("dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx): %w", err)
			}
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(h))
			if err != nil {
				return fmt.Errorf("html.Parse(strings.NewReader(renderedHTML)): %w", err)
			}

			doc.Find("dl.foodArray").Each(func(i int, selection *goquery.Selection) {
				m := Menu{}

				noDelivery := selection.Find("span.no-delivery").Text()
				if noDelivery != "" {
					// no-delivery is true, ignore
					return
				}

				a := selection.Find("a.modalOpenButton")
				modalID, exist := a.Attr("data-izimodal-open")
				if exist {
					menuID := strings.TrimPrefix(modalID, "#modal-")
					id, err := strconv.Atoi(menuID)
					if err != nil {
						return
					}
					m.ID = id
				}

				img, exist := a.Find("img").Attr("src")
				if exist {
					u, err := url.Parse(img)
					if err != nil {
						return
					}
					m.ImageURL = u
				}

				name := selection.Find("p.name").Text()
				m.Name = name

				nu, err := getNutrition(selection)
				if err != nil {
					return
				}
				m.Nutrition = *nu

				c := strings.TrimSuffix(selection.Find("span.count").Text(), "食")
				if c != "" {
					count, err := strconv.Atoi(c)
					if err != nil {
						return
					}
					m.Count = count
				}

				menus = append(menus, m)
			})

			return nil
		}),
	}); err != nil {
		return nil, fmt.Errorf("chromedp.Run(ctx): %w", err)
	}

	return menus, nil
}

func getNutrition(selection *goquery.Selection) (*Nutrition, error) {
	sugarAttr := selection.AttrOr("sugar", "0")
	sugar, err := strconv.ParseFloat(sugarAttr, 10)
	if err != nil {
		return nil, fmt.Errorf("strconv.ParseFloat(%s, 10)", sugarAttr)
	}

	salinityAttr := selection.AttrOr("salinity", "0")
	sality, err := strconv.ParseFloat(salinityAttr, 10)
	if err != nil {
		return nil, fmt.Errorf("strconv.ParseFloat(%s, 10)", salinityAttr)
	}

	calorieAttr := selection.AttrOr("calories", "0")
	calorie, err := strconv.ParseFloat(calorieAttr, 10)
	if err != nil {
		return nil, fmt.Errorf("strconv.ParseFloat(%s, 10)", calorieAttr)
	}

	proteinAttr := selection.AttrOr("protein", "0")
	protein, err := strconv.ParseFloat(proteinAttr, 10)
	if err != nil {
		return nil, fmt.Errorf("strconv.ParseFloat(%s, 10)", proteinAttr)
	}

	fiberAttr := selection.AttrOr("fiber", "0")
	fiber, err := strconv.ParseFloat(fiberAttr, 10)
	if err != nil {
		return nil, fmt.Errorf("strconv.ParseFloat(%s, 10)", fiberAttr)
	}

	lipidAttr := selection.AttrOr("lipid", "0")
	lipid, err := strconv.ParseFloat(lipidAttr, 10)
	if err != nil {
		return nil, fmt.Errorf("strconv.ParseFloat(%s, 10)", lipidAttr)
	}

	return &Nutrition{
		Sugar:    sugar,
		Salinity: sality,
		Calorie:  calorie,
		Protein:  protein,
		Fiber:    fiber,
		Lipid:    lipid,
	}, nil
}
