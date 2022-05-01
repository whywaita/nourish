package nosh

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/PuerkitoBio/goquery"
)

var (
	// JST is timezone for Asia/Tokyo
	JST = time.FixedZone("Asia/Tokyo", 9*60*60)
)

// GetSchedule get schedules in now and next month
func GetSchedule(ctx context.Context, cookies []*network.Cookie, userID int, logger *zap.Logger) ([]ScheduleNode, []ScheduleNode, []ScheduleNode, error) {
	now := time.Now()

	year := now.Year()
	month := now.Month()
	deadline, skip, delivery, err := GetScheduleMonth(ctx, cookies, userID, year, int(month), logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("GetScheduleMonth(ctx, cookies, userID, %d, %d): %w", year, month, err)
	}

	ndeadline, nskip, ndelivery, err := GetScheduleMonth(ctx, cookies, userID, year, int(month+1), logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("GetScheduleMonth(ctx, cookies, userID, %d, %d): %w", year, month, err)
	}

	deadline = mergeSchedules(ndeadline, deadline)
	skip = mergeSchedules(nskip, skip)
	delivery = mergeSchedules(ndelivery, delivery)
	return deadline, skip, delivery, nil
}

// GetScheduleMonth get schedules in year/month
func GetScheduleMonth(ctx context.Context, cookies []*network.Cookie, userID int, year, month int, logger *zap.Logger) ([]ScheduleNode, []ScheduleNode, []ScheduleNode, error) {
	var deadline, skip, delivery []ScheduleNode

	if err := chromedp.Run(ctx, chromedp.Tasks{
		SetCookiesAction(cookies),
		chromedp.Navigate(fmt.Sprintf("%s/mypage/subscription/%d?month=%d-%02d", BaseAPI, userID, year, month)),
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

			deadline, skip, delivery, err = ParseCalendar(doc, year, month, logger)
			if err != nil {
				return fmt.Errorf("extractCalendar(doc, %d, %d): %w", year, month, err)
			}

			return nil
		}),
	}); err != nil {
		return nil, nil, nil, fmt.Errorf("chromedp.Run(ctx): %w", err)
	}

	return deadline, skip, delivery, nil
}

// ScheduleType is type for schedule
type ScheduleType int

// ScheduleType iota
const (
	ScheduleTypeUnknown ScheduleType = iota
	ScheduleTypeDeadline
	ScheduleTypeSkip
	ScheduleTypeDelivery
)

// String implement fmt.Stringer
func (in ScheduleType) String() string {
	switch in {
	case ScheduleTypeDeadline:
		return "deadline"
	case ScheduleTypeSkip:
		return "skip"
	case ScheduleTypeDelivery:
		return "delivery"
	default:
		return "unknown"
	}
}

// Class names
const (
	ClassDeadline = "dt.date--deadline"
	ClassSkip     = "dt.date--plan-skip"
	ClassDelivery = "dt.date--confirm-delivery"
)

// MarshalScheduleType marshal from class name
func MarshalScheduleType(in string) ScheduleType {
	switch in {
	case ClassDeadline:
		return ScheduleTypeDeadline
	case ClassSkip:
		return ScheduleTypeSkip
	case ClassDelivery:
		return ScheduleTypeDelivery
	default:
		return ScheduleTypeUnknown
	}
}

// ScheduleNode is node of Schedule
type ScheduleNode struct {
	ScheduleID int
	Type       ScheduleType
	Date       time.Time
	Link       string

	// for ScheduleTypeDeadline
	DeliveryDate     *time.Time
	WillScheduleType *ScheduleType
}

// ParseCalendar parse calendar
func ParseCalendar(root *goquery.Document, year, month int, logger *zap.Logger) ([]ScheduleNode, []ScheduleNode, []ScheduleNode, error) {
	var deadline, skip, delivery []ScheduleNode

	root.Find(ClassDeadline).Each(func(i int, selection *goquery.Selection) {
		sn, err := toScheduleNode(selection, MarshalScheduleType(ClassDeadline), year, month)
		if err != nil {
			logger.Info("toScheduleNode()", zap.Error(err))
			return
		}
		deadline = append(deadline, *sn)
	})

	root.Find(ClassSkip).Each(func(i int, selection *goquery.Selection) {
		sn, err := toScheduleNode(selection, MarshalScheduleType(ClassSkip), year, month)
		if err != nil {
			logger.Info("toScheduleNode()", zap.Error(err))
			return
		}
		skip = append(skip, *sn)
	})

	root.Find(ClassDelivery).Each(func(i int, selection *goquery.Selection) {
		sn, err := toScheduleNode(selection, MarshalScheduleType(ClassDelivery), year, month)
		if err != nil {
			logger.Info("toScheduleNode()", zap.Error(err))
			return
		}
		delivery = append(delivery, *sn)
	})

	deadline = fillDeadline(deadline, skip, delivery)

	return deadline, skip, delivery, nil
}

func toScheduleNode(selection *goquery.Selection, stType ScheduleType, year, month int) (*ScheduleNode, error) {
	dl := selection.Parent()
	if dl == nil {
		return nil, fmt.Errorf("parent selection is nil")
	}

	a := dl.Parent()
	link, found := a.Attr("href")
	if !found {
		return nil, fmt.Errorf("parent select is not a")
	}

	scheduleID, err := getScheduleID(link)
	if err != nil {
		return nil, fmt.Errorf("getScheduleID(%s): %w", link, err)
	}

	day, err := strconv.Atoi(strings.TrimSpace(selection.Text()))
	if err != nil {
		return nil, fmt.Errorf("strconv.Atoi(%s): %w", strings.TrimSpace(selection.Text()), err)
	}

	var deliveryDate *time.Time
	deliveryDate = nil
	if stType == ScheduleTypeDeadline {
		pText := strings.TrimSpace(dl.Find("p.schedule-daybox__desc").Text())
		pText = strings.ReplaceAll(pText, "\n", "")
		pText = strings.ReplaceAll(pText, " ", "")
		dateText := strings.Trim(pText, "変更締切")
		layout := "1月2日"
		t, err := time.Parse(layout, dateText)
		if err != nil {
			return nil, fmt.Errorf("time.Parse(%s, %s): %w", layout, dateText, err)
		}
		d := time.Date(year, t.Month(), t.Day(), 0, 0, 0, 0, JST)
		deliveryDate = &d
	}

	return &ScheduleNode{
		ScheduleID:   scheduleID,
		Type:         stType,
		Date:         time.Date(year, time.Month(month), day, 0, 0, 0, 0, JST),
		DeliveryDate: deliveryDate,
		Link:         link,
	}, nil
}

func getScheduleID(scheduleURL string) (int, error) {
	// scheduleURL: https://nosh.jp/mypage/${userID}/${scheduleID}
	u := strings.Split(scheduleURL, "/")
	scheduleID := u[len(u)-1]
	id, err := strconv.Atoi(scheduleID)
	if err != nil {
		return -1, fmt.Errorf("strconv.Atoi(%s): %w", scheduleID, err)
	}
	return id, nil
}

func mergeSchedules(n, m []ScheduleNode) []ScheduleNode {
	registered := map[int]bool{}
	var sn []ScheduleNode

	for _, val := range n {
		sn = append(sn, val)
		registered[val.ScheduleID] = true
	}

	for _, val := range m {
		if _, ok := registered[val.ScheduleID]; ok {
			// is appended, will not append
		} else {
			sn = append(sn, val)
		}
	}

	return sn
}

func fillDeadline(deadline, skip, delivery []ScheduleNode) []ScheduleNode {
	deliveryDay := map[string]ScheduleNode{}
	for _, s := range skip {
		deliveryDay[s.Link] = s
	}
	for _, d := range delivery {
		deliveryDay[d.Link] = d
	}

	var filled []ScheduleNode
	for _, d := range deadline {
		dd := deliveryDay[d.Link]

		filled = append(filled, ScheduleNode{
			ScheduleID:       d.ScheduleID,
			Type:             d.Type,
			Date:             d.Date,
			Link:             d.Link,
			DeliveryDate:     d.DeliveryDate,
			WillScheduleType: &dd.Type,
		})
	}

	return filled
}
