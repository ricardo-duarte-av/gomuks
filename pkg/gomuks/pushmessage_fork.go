// gomuks - A Matrix client written in Go.
// Copyright (C) 2025 Tulir Asokan
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

//go:build !js

package gomuks

// Push notification formatting that only exists in this fork. It's kept out of pushmessage.go
// so that upstream changes to that file merge cleanly: pushmessage.go only has the RTC field on
// PushNewMessage and a single call to formatForkPushNotification.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog"
	"go.mau.fi/util/jsontime"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/pkg/hicli/jsoncmd"
)

// isImportant returns whether the notification should be delivered at high priority.
func (pnm *PushNewMessage) isImportant() bool {
	return pnm.Sound || (pnm.RTC != nil && pnm.RTC.Type == rtcNotificationTypeRing)
}

// PushRTCNotification is the call metadata included in pushes for MSC4075 RTC notifications.
// Clients use it to show call UI instead of a regular message notification.
type PushRTCNotification struct {
	// Type is the notification_type of the event, passed through as-is. Usually "ring" or "notification".
	Type string `json:"type"`
	// Intent is the m.call.intent of the event, if present. Usually "video" or "audio".
	Intent string `json:"intent,omitempty"`
	// CallID is the event ID of the RTC session the notification references, if present.
	CallID id.EventID `json:"call_id,omitempty"`
	// ExpiresAt is when the notification stops being relevant, if the event declared a lifetime.
	// It's calculated from the sender's clock, which may be skewed from both the server and the client.
	ExpiresAt *jsontime.UnixMilli `json:"expires_at,omitempty"`
}

// formatForkPushNotification formats pushes for event types that upstream doesn't notify for.
// handled is false for event types that should go through the upstream formatting.
func (gmx *Gomuks) formatForkPushNotification(
	ctx context.Context, notif jsoncmd.SyncNotification, evtType event.Type, rawContent json.RawMessage,
) (msg *PushNewMessage, handled bool) {
	switch evtType {
	case event.EventReaction:
		text := gmx.formatReactionNotificationText(ctx, notif, rawContent)
		if text == "" {
			return nil, true
		}
		return gmx.newForkPushMessage(ctx, notif, text, false), true
	case EventRTCNotification:
		return gmx.formatRTCNotification(ctx, notif, rawContent), true
	default:
		return nil, false
	}
}

// newForkPushMessage builds a push with the given text by running a plain text event through
// the upstream formatting, so that fork notifications get every field upstream fills (room info,
// sender, DM flag, etc) without duplicating that code here.
func (gmx *Gomuks) newForkPushMessage(
	ctx context.Context, notif jsoncmd.SyncNotification, text string, mention bool,
) *PushNewMessage {
	content, err := json.Marshal(&event.MessageEventContent{MsgType: event.MsgText, Body: text})
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("Failed to marshal stand-in content for push notification")
		return nil
	}
	evt := *notif.Event
	evt.Type = event.EventMessage.Type
	evt.StateKey = nil
	evt.Content = content
	evt.Decrypted = nil
	evt.DecryptedType = ""
	evt.LocalContent = &database.LocalContent{PreviewText: text}
	notif.Event = &evt
	msg := gmx.formatPushNotificationMessage(ctx, notif)
	if msg != nil {
		msg.Mention = mention
	}
	return msg
}

// reactionTargetPreviewLength is the maximum length of the reacted-to message quoted in a reaction notification.
const reactionTargetPreviewLength = 80

func reactionKeyForNotification(key string) string {
	// Custom emoji reactions use an mxc:// URI as the key, which isn't worth showing as-is.
	if strings.HasPrefix(key, "mxc://") {
		return "with a custom emoji"
	}
	if utf8.RuneCountInString(key) > 16 {
		return string([]rune(key)[:16]) + "…"
	}
	return key
}

func (gmx *Gomuks) formatReactionNotificationText(ctx context.Context, notif jsoncmd.SyncNotification, rawContent json.RawMessage) string {
	var content event.ReactionEventContent
	err := json.Unmarshal(rawContent, &content)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).
			Stringer("event_id", notif.Event.ID).
			Msg("Failed to unmarshal reaction content to format push notification")
		return ""
	}
	if content.RelatesTo.Key == "" || content.RelatesTo.EventID == "" {
		return ""
	}
	key := reactionKeyForNotification(content.RelatesTo.Key)
	target, err := gmx.Client.DB.Event.GetByID(ctx, notif.Room.ID, content.RelatesTo.EventID)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).
			Stringer("event_id", content.RelatesTo.EventID).
			Msg("Failed to get reaction target for push notification")
	}
	fallback := "a message"
	if target != nil && target.Sender == gmx.Client.Account.UserID {
		fallback = "your message"
	}
	// Preview text is only generated for messages and stickers, so this doubles as an
	// event type filter. It also follows edits, so the quote matches the current text.
	var targetText string
	if target != nil {
		if localContent := target.GetLocalContent(); localContent != nil {
			targetText = localContent.PreviewText
		}
	}
	if targetText == "" {
		return fmt.Sprintf("Reacted %s to %s", key, fallback)
	}
	if utf8.RuneCountInString(targetText) > reactionTargetPreviewLength {
		targetText = string([]rune(targetText)[:reactionTargetPreviewLength]) + "…"
	}
	return fmt.Sprintf("Reacted %s to %q", key, targetText)
}

// EventRTCNotification is the MSC4075 event type for RTC (call) notifications.
var EventRTCNotification = event.Type{Type: "org.matrix.msc4075.rtc.notification", Class: event.MessageEventType}

const (
	rtcNotificationTypeRing   = "ring"
	rtcNotificationTypeNotify = "notification"
)

type rtcNotificationContent struct {
	NotificationType string           `json:"notification_type"`
	Intent           string           `json:"m.call.intent"`
	SenderTS         int64            `json:"sender_ts"`
	Lifetime         int64            `json:"lifetime"`
	RelatesTo        *event.RelatesTo `json:"m.relates_to"`
	Mentions         *event.Mentions  `json:"m.mentions"`
}

func rtcNotificationText(notificationType, intent string) string {
	video := intent == "video"
	if notificationType == rtcNotificationTypeRing {
		if video {
			return "Incoming video call"
		}
		return "Incoming call"
	}
	if video {
		return "Started a video call"
	}
	return "Started a call"
}

// rtcNotificationExpiry calculates when an RTC notification stops being relevant.
// sender_ts is the sender's clock rather than the server's, but it's what the lifetime is
// relative to. Senders that don't include it fall back to the event timestamp.
func rtcNotificationExpiry(senderTS, lifetime, eventTS int64) (time.Time, bool) {
	if lifetime <= 0 {
		return time.Time{}, false
	}
	if senderTS == 0 {
		senderTS = eventTS
	}
	return time.UnixMilli(senderTS + lifetime), true
}

func (gmx *Gomuks) formatRTCNotification(ctx context.Context, notif jsoncmd.SyncNotification, rawContent json.RawMessage) *PushNewMessage {
	var content rtcNotificationContent
	err := json.Unmarshal(rawContent, &content)
	if err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).
			Stringer("event_id", notif.Event.ID).
			Msg("Failed to unmarshal RTC notification content to format push notification")
		return nil
	}
	if content.NotificationType == "" {
		return nil
	}
	rtc := &PushRTCNotification{
		Type:   content.NotificationType,
		Intent: content.Intent,
	}
	if content.RelatesTo != nil && content.RelatesTo.Type == event.RelReference {
		rtc.CallID = content.RelatesTo.EventID
	}
	if expiry, ok := rtcNotificationExpiry(content.SenderTS, content.Lifetime, notif.Event.Timestamp.UnixMilli()); ok {
		if expiry.Before(time.Now()) {
			// Syncing after being offline for a while can replay call notifications that are long
			// dead, and there's nothing useful to show for those.
			zerolog.Ctx(ctx).Debug().
				Stringer("event_id", notif.Event.ID).
				Time("expiry", expiry).
				Msg("Skipping push for expired RTC notification")
			return nil
		}
		rtc.ExpiresAt = ptr.Ptr(jsontime.UM(expiry))
	}
	msg := gmx.newForkPushMessage(
		ctx, notif, rtcNotificationText(content.NotificationType, content.Intent),
		content.Mentions.Has(gmx.Client.Account.UserID),
	)
	if msg != nil {
		msg.RTC = rtc
	}
	return msg
}
