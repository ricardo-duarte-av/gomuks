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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"maunium.net/go/mautrix/id"
)

func makeDismisses(count int) []PushDismiss {
	dismisses := make([]PushDismiss, count)
	for i := range dismisses {
		dismisses[i] = PushDismiss{
			RoomID: id.RoomID(fmt.Sprintf("!room%02d:example.com", i)),
		}
	}
	return dismisses
}

func makeMessages(count, size int) ([]json.RawMessage, []*PushNewMessage) {
	raw := make([]json.RawMessage, count)
	orig := make([]*PushNewMessage, count)
	for i := range raw {
		raw[i] = json.RawMessage(fmt.Sprintf(`{"text":"%s"}`, strings.Repeat("a", size)))
		orig[i] = &PushNewMessage{}
	}
	return raw, orig
}

func collectSplit(pn *PushNotification) []*PushNotification {
	var out []*PushNotification
	for chunk := range pn.Split {
		out = append(out, chunk)
	}
	return out
}

func TestPushNotificationSplit_DismissOnly(t *testing.T) {
	pn := &PushNotification{Dismiss: makeDismisses(1)}
	chunks := collectSplit(pn)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 push, got %d", len(chunks))
	}
	if len(chunks[0].Dismiss) != 1 {
		t.Errorf("expected 1 dismissal, got %d", len(chunks[0].Dismiss))
	}
}

func TestPushNotificationSplit_ManyDismisses(t *testing.T) {
	dismisses := makeDismisses(25)
	raw, orig := makeMessages(6, 600)
	pn := &PushNotification{
		Dismiss:      slices.Clone(dismisses),
		RawMessages:  raw,
		OrigMessages: orig,
		ImageAuth:    "meow",
	}
	chunks := collectSplit(pn)
	if len(chunks) < 2 {
		t.Fatalf("expected the messages to be split into multiple pushes, got %d", len(chunks))
	}

	var gotDismisses []PushDismiss
	var gotMessages int
	for i, chunk := range chunks {
		gotDismisses = append(gotDismisses, chunk.Dismiss...)
		gotMessages += len(chunk.RawMessages)
		payload, err := json.Marshal(chunk)
		if err != nil {
			t.Fatalf("failed to marshal push %d: %v", i, err)
		}
		// Same limit as SendPushNotification
		if encodedLen := base64.StdEncoding.EncodedLen(len(payload)); encodedLen >= 4000 {
			t.Errorf("push %d is too long (%d encoded bytes)", i, encodedLen)
		}
	}
	if gotMessages != len(raw) {
		t.Errorf("expected %d messages across all pushes, got %d", len(raw), gotMessages)
	}
	if !slices.Equal(gotDismisses, dismisses) {
		t.Errorf("expected each dismissal exactly once in order, got %+v", gotDismisses)
	}
}

func TestPushNotificationSplit_DismissesWithoutMessages(t *testing.T) {
	dismisses := makeDismisses(60)
	pn := &PushNotification{Dismiss: slices.Clone(dismisses)}
	chunks := collectSplit(pn)
	if len(chunks) < 2 {
		t.Fatalf("expected the dismissals to be split into multiple pushes, got %d", len(chunks))
	}
	var gotDismisses []PushDismiss
	for _, chunk := range chunks {
		if len(chunk.RawMessages) != 0 {
			t.Error("expected dismiss-only pushes to have no messages")
		}
		gotDismisses = append(gotDismisses, chunk.Dismiss...)
	}
	if !slices.Equal(gotDismisses, dismisses) {
		t.Errorf("expected each dismissal exactly once in order, got %d dismissals", len(gotDismisses))
	}
}
