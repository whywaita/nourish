package notify

import (
	"context"
	"fmt"
	"time"

	"github.com/slack-go/slack"
	"github.com/whywaita/nourish/pkg/nosh"
)

var (
	// ErrNoNeedPostType is error message for not nosh.ScheduleTypeDelivery
	ErrNoNeedPostType = fmt.Errorf("not nosh.ScheduleTypeDelivery")
)

const (
	// Username is username of slack
	Username = "nourish"
	// IconURL is URL of icon
	IconURL = "https://1.bp.blogspot.com/-VdRARu0Xvm0/Xlyf8ZzqClI/AAAAAAABXrI/fjsmV2v7UB0UHJzmXAfB-7zjXFvxJx9QgCNcBGAsYHQ/s1600/pulp_mold_obentou.png"
)

// RemindDeadline post menus to slack
func RemindDeadline(ctx context.Context, webhookURL, channelName string, menus []nosh.Menu, deadlineSchedule nosh.ScheduleNode) error {
	if deadlineSchedule.Type != nosh.ScheduleTypeDeadline {
		return fmt.Errorf("%s is not ScheduleTypeDeadline", deadlineSchedule.Type)
	}
	if *deadlineSchedule.WillScheduleType != nosh.ScheduleTypeDelivery {
		return fmt.Errorf("%s is not nosh.ScheduleTypeDelivery, not need post: %w", *deadlineSchedule.WillScheduleType, ErrNoNeedPostType)
	}

	text := fmt.Sprintf(`メニュー変更締切が迫っています (締切: %s 受取: %s)
%s
`,
		prettyTime(deadlineSchedule.Date),
		prettyTime(*deadlineSchedule.DeliveryDate),
		deadlineSchedule.Link)
	for _, menu := range menus {
		text += fmt.Sprintf(`
- %v`, menu.PrettyString())
	}

	wm := slack.WebhookMessage{
		Username: Username,
		IconURL:  IconURL,
		Channel:  channelName,
		Text:     text,
	}

	if err := slack.PostWebhookContext(ctx, webhookURL, &wm); err != nil {
		return fmt.Errorf("slack.PostWebhookContext(ctx, webhookURL, %v): %w", wm, err)
	}
	return nil
}

func prettyTime(in time.Time) string {
	return fmt.Sprintf("%d/%d/%d", in.Year(), in.Month(), in.Day())
}
