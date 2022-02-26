package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/whywaita/nourish/pkg/notify"

	"github.com/whywaita/nourish/pkg/nosh"

	"github.com/chromedp/chromedp"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

var (
	c = conf{}
)

type conf struct {
	Email    string
	Password string

	SlackWebhookURL  string
	SlackChannelName string

	RemindHour float64
}

func init() {
	if os.Getenv("NOURISH_EMAIL") == "" || os.Getenv("NOURISH_PASSWORD") == "" {
		log.Panic("need to set env")
	}
	if os.Getenv("NOURISH_SLACK_URL") == "" || os.Getenv("NOURISH_SLACK_CHANNEL") == "" {
		log.Panic("need to set env")
	}

	c.Email = os.Getenv("NOURISH_EMAIL")
	c.Password = os.Getenv("NOURISH_PASSWORD")
	c.SlackWebhookURL = os.Getenv("NOURISH_SLACK_URL")
	c.SlackChannelName = os.Getenv("NOURISH_SLACK_CHANNEL")

	rh := os.Getenv("NOURISH_REMIND_HOUR")
	if rh == "" {
		rh = "24"
	}
	remindHour, err := strconv.ParseFloat(rh, 10)
	if err != nil {
		log.Panicf("NOURISH_REMIND_HOUR is invalid format (%s): %+v", rh, err)
	}
	c.RemindHour = remindHour
}

func needRemindDeadline(schedules []nosh.ScheduleNode) []nosh.ScheduleNode {
	var need []nosh.ScheduleNode

	for _, schedule := range schedules {
		until := time.Until(*schedule.DeadlineDate)
		if until.Hours() < c.RemindHour {
			need = append(need, schedule)
		}
	}

	return need
}

func run() error {
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		//chromedp.WithDebugf(log.Printf),
	)
	defer cancel()

	identity, err := nosh.Login(ctx, c.Email, c.Password)
	if err != nil {
		return fmt.Errorf("login(ctx): %w", err)
	}

	deadline, _, _, err := nosh.GetSchedule(ctx, identity.Cookies, identity.UserID)
	if err != nil {
		return fmt.Errorf("GetSchedule(ctx, cookie, %d): %w", identity.UserID, err)
	}
	needRemind := needRemindDeadline(deadline)
	if len(needRemind) == 0 {
		log.Println("no need remind")
		return nil
	}

	for _, schedule := range needRemind {
		menus, err := nosh.GetMenuByScheduleID(ctx, identity.Cookies, identity.UserID, schedule.ScheduleID)
		if err != nil {
			return fmt.Errorf("GetMenuByScheduleID(ctx, cookies, %d, %d): %w", identity.UserID, schedule.ScheduleID, err)
		}

		err = notify.RemindDeadline(ctx, c.SlackWebhookURL, c.SlackChannelName, menus, schedule)
		if err != nil && !errors.Is(err, notify.ErrNoNeedPostType) {
			return fmt.Errorf("notify.RemindDeadline(ctx, webhookURL, channelName, menu, schedule): %w", err)
		}
	}

	fmt.Println("done")
	return nil
}
