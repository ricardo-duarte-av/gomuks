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

func TestRTCNotificationText(t *testing.T) {
	tests := []struct {
		name             string
		notificationType string
		intent           string
		expected         string
	}{
		{name: "ring video", notificationType: "ring", intent: "video", expected: "Incoming video call"},
		{name: "ring audio", notificationType: "ring", intent: "audio", expected: "Incoming call"},
		{name: "ring without intent", notificationType: "ring", expected: "Incoming call"},
		{name: "notification video", notificationType: "notification", intent: "video", expected: "Started a video call"},
		{name: "notification audio", notificationType: "notification", intent: "audio", expected: "Started a call"},
		{name: "unknown type falls back", notificationType: "something_new", expected: "Started a call"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rtcNotificationText(test.notificationType, test.intent); got != test.expected {
				t.Errorf("rtcNotificationText(%q, %q) = %q, want %q", test.notificationType, test.intent, got, test.expected)
			}
		})
	}
}

func TestRTCNotificationExpiry(t *testing.T) {
	const eventTS = 1789036969729
	const senderTS = 1789036970949
	tests := []struct {
		name       string
		senderTS   int64
		lifetime   int64
		expectedOK bool
		expectedMS int64
	}{
		{name: "sender ts is preferred", senderTS: senderTS, lifetime: 30000, expectedOK: true, expectedMS: senderTS + 30000},
		{name: "falls back to event ts", lifetime: 30000, expectedOK: true, expectedMS: eventTS + 30000},
		{name: "no lifetime", senderTS: senderTS, expectedOK: false},
		{name: "negative lifetime", senderTS: senderTS, lifetime: -1, expectedOK: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expiry, ok := rtcNotificationExpiry(test.senderTS, test.lifetime, eventTS)
			if ok != test.expectedOK {
				t.Fatalf("rtcNotificationExpiry(%d, %d, %d) ok = %v, want %v", test.senderTS, test.lifetime, eventTS, ok, test.expectedOK)
			}
			if ok && expiry.UnixMilli() != test.expectedMS {
				t.Errorf("rtcNotificationExpiry(%d, %d, %d) = %d, want %d", test.senderTS, test.lifetime, eventTS, expiry.UnixMilli(), test.expectedMS)
			}
		})
	}
}
