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
	"context"
	"testing"

	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/jsoncmd"
)

func TestInvitePushDismissedOnceAnswered(t *testing.T) {
	gmx := &Gomuks{}
	joined := id.RoomID("!joined:example.com")
	declined := id.RoomID("!declined:example.com")
	other := id.RoomID("!other:example.com")
	invitePushed.Store(joined, struct{}{})
	invitePushed.Store(declined, struct{}{})
	t.Cleanup(func() { invitePushed.Clear() })

	push := PushNotification{Dismiss: []PushDismiss{{RoomID: joined, ReadUpTo: "$read"}}}
	gmx.addForkInvitePushes(context.Background(), &jsoncmd.SyncComplete{
		Rooms:     map[id.RoomID]*jsoncmd.SyncRoom{joined: {}, other: {}},
		LeftRooms: []id.RoomID{declined},
	}, &push)

	// The joined room already had a dismissal: not repeated. The declined one gets one. A room
	// whose invite wasn't pushed gets nothing.
	want := []PushDismiss{{RoomID: joined, ReadUpTo: "$read"}, {RoomID: declined}}
	if len(push.Dismiss) != len(want) {
		t.Fatalf("got dismissals %+v, want %+v", push.Dismiss, want)
	}
	for i := range want {
		if push.Dismiss[i] != want[i] {
			t.Fatalf("got dismissals %+v, want %+v", push.Dismiss, want)
		}
	}
	if _, ok := invitePushed.Load(joined); ok {
		t.Fatal("answered invite still remembered")
	}

	// Answered once: a later sync of the same room doesn't dismiss again.
	push = PushNotification{}
	gmx.addForkInvitePushes(context.Background(), &jsoncmd.SyncComplete{
		Rooms: map[id.RoomID]*jsoncmd.SyncRoom{joined: {}},
	}, &push)
	if len(push.Dismiss) != 0 {
		t.Fatalf("got dismissals %+v, want none", push.Dismiss)
	}
}
