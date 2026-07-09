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

import (
	"testing"

	"maunium.net/go/mautrix/event"
)

func TestReactionKeyForNotification(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{name: "simple emoji", key: "❤️", expected: "❤️"},
		{name: "multi-codepoint emoji", key: "👨‍👩‍👧‍👦", expected: "👨‍👩‍👧‍👦"},
		{name: "short text", key: "lgtm", expected: "lgtm"},
		{name: "custom emoji", key: "mxc://example.com/abc123", expected: "with a custom emoji"},
		{name: "long text is truncated by runes", key: "äääääääääääääääääää", expected: "ääääääääääääääää…"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := reactionKeyForNotification(test.key); got != test.expected {
				t.Errorf("reactionKeyForNotification(%q) = %q, want %q", test.key, got, test.expected)
			}
		})
	}
}

func TestMessageBodyForNotification(t *testing.T) {
	tests := []struct {
		name     string
		content  event.MessageEventContent
		expected string
	}{{
		name:     "text message",
		content:  event.MessageEventContent{MsgType: event.MsgText, Body: "hello"},
		expected: "hello",
	}, {
		name:     "image without filename",
		content:  event.MessageEventContent{MsgType: event.MsgImage, Body: "image.png"},
		expected: "Sent an image",
	}, {
		name:     "sticker",
		content:  event.MessageEventContent{MsgType: event.CapMsgSticker, Body: "sticker"},
		expected: "Sent a sticker",
	}, {
		name:     "voice message",
		content:  event.MessageEventContent{MsgType: event.MsgAudio, Body: "audio.ogg", MSC3245Voice: &event.MSC3245Voice{}},
		expected: "Sent a voice message",
	}, {
		name:     "audio file",
		content:  event.MessageEventContent{MsgType: event.MsgAudio, Body: "audio.ogg"},
		expected: "Sent an audio file",
	}, {
		name:     "image with caption keeps caption",
		content:  event.MessageEventContent{MsgType: event.MsgImage, Body: "look at this", FileName: "image.png"},
		expected: "look at this",
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := messageBodyForNotification(&test.content); got != test.expected {
				t.Errorf("messageBodyForNotification() = %q, want %q", got, test.expected)
			}
		})
	}
}
