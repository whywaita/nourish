package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/bluele/zapslack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/chromedp/chromedp"
	"github.com/whywaita/nourish/pkg/nosh"
	"github.com/whywaita/nourish/pkg/notify"
)

func main() {
	logConf := zap.NewProductionConfig()
	logConf.OutputPaths = []string{"stdout"}
	logConf.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, err := logConf.Build()
	if err != nil {
		log.Fatalf("failed to zap.NewProduction: %+v", err)
	}
	sh := &zapslack.SlackHook{
		HookURL: c.SlackWebhookURL,
		AcceptedLevels: []zapcore.Level{
			zapcore.InfoLevel,
			zapcore.WarnLevel,
			zapcore.ErrorLevel,
		},
		Username: notify.Username,
		IconURL:  notify.IconURL,
	}

	logger = logger.WithOptions(
		// no notification in debug level
		zap.Hooks(sh.GetHook()),
	)

	if err := run(logger); err != nil {
		logger.Error(err.Error())
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
	remindHour, err := strconv.ParseFloat(rh, 64)
	if err != nil {
		log.Panicf("NOURISH_REMIND_HOUR is invalid format (%s): %+v", rh, err)
	}
	c.RemindHour = remindHour
}

func needRemindDeadline(schedules []nosh.ScheduleNode) []nosh.ScheduleNode {
	var need []nosh.ScheduleNode

	for _, schedule := range schedules {
		// not need remind if not deadline
		if schedule.Type != nosh.ScheduleTypeDeadline {
			continue
		}

		// not need remind if not deadline of delivery
		if *schedule.WillScheduleType != nosh.ScheduleTypeDelivery {
			continue
		}

		until := time.Until(schedule.Date)
		if until.Hours() < c.RemindHour {
			need = append(need, schedule)
		}
	}

	return need
}

func run(logger *zap.Logger) error {
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		//chromedp.WithDebugf(log.Printf),
	)
	defer cancel()

	ctx, timeout := context.WithTimeout(ctx, 5*time.Minute)
	defer timeout()

	logger.Debug("start login...")
	identity, err := nosh.Login(ctx, c.Email, c.Password)
	if err != nil {
		return fmt.Errorf("login(ctx): %w", err)
	}
	logger.Debug("login successfully")

	deadline, _, _, err := nosh.GetSchedule(ctx, identity.Cookies, identity.UserID, logger)
	if err != nil {
		return fmt.Errorf("GetSchedule(ctx, cookie, %d): %w", identity.UserID, err)
	}
	needRemind := needRemindDeadline(deadline)
	if len(needRemind) == 0 {
		logger.Debug("no need remind")
		return nil
	}

	logger.Debug("will notify deadline", zap.Any("schedules", needRemind))

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

	logger.Debug("remind done")
	return nil
}
