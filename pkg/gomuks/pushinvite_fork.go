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

// Invite pushes, which only exist in this fork. Upstream only pushes timeline events of joined
// rooms, so being invited to a room (or a DM) went unnoticed until the client was opened. They're
// sent under "invites" in the push payload, each invite as a push of its own, and kept out of
// push.go apart from a couple of calls so that upstream changes merge cleanly.

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/util/jsontime"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/gomuks/pkg/hicli/database"
	"go.mau.fi/gomuks/pkg/hicli/jsoncmd"
)

// PushInvite is an invite to a room, as pushed. It's built from the invite's stripped state, which
// comes from the inviter's server and is unverified until the room is joined.
type PushInvite struct {
	// Timestamp is when we were invited (or, failing that, when the invite was received).
	Timestamp jsontime.UnixMilli `json:"timestamp"`
	RoomID    id.RoomID          `json:"room_id"`
	// RoomName is the room's name or canonical alias, empty if it has neither (a DM, usually).
	RoomName   string `json:"room_name,omitempty"`
	RoomAvatar string `json:"room_avatar,omitempty"`
	// IsDM is set when the inviter marked it a DM (is_direct on our invite).
	IsDM    bool             `json:"is_dm,omitzero"`
	IsSpace bool             `json:"is_space,omitzero"`
	Inviter NotificationUser `json:"inviter"`
	Self    NotificationUser `json:"self"`
	// Reason is why we were invited, if the inviter said.
	Reason string `json:"reason,omitempty"`
}

// invitePushed holds the rooms whose invites were pushed, so that the push is dismissed once the
// invite is accepted or declined (from any client). It's only kept in memory: after a restart,
// clients still drop the notification when they see the room joined or left themselves.
var invitePushed sync.Map

// inviteProfileTimeout bounds fetching the inviter's profile when the invite state doesn't have it.
const inviteProfileTimeout = 5 * time.Second

// maxInviteReasonLength is how much of the invite's reason is pushed.
const maxInviteReasonLength = 200

// addForkInvitePushes formats the sync's new invites into push.RawInvites, and dismisses the pushes
// of invites that were answered (the room was joined or left).
func (gmx *Gomuks) addForkInvitePushes(ctx context.Context, sync *jsoncmd.SyncComplete, push *PushNotification) {
	answered := func(roomID id.RoomID) {
		if _, ok := invitePushed.LoadAndDelete(roomID); !ok {
			return
		}
		if !slices.ContainsFunc(push.Dismiss, func(d PushDismiss) bool { return d.RoomID == roomID }) {
			push.Dismiss = append(push.Dismiss, PushDismiss{RoomID: roomID})
		}
	}
	for roomID := range sync.Rooms {
		answered(roomID)
	}
	for _, roomID := range sync.LeftRooms {
		answered(roomID)
	}
	for _, invite := range sync.InvitedRooms {
		msg := gmx.formatInvitePush(ctx, invite)
		if msg == nil {
			continue
		}
		raw, err := json.Marshal(msg)
		if err != nil {
			zerolog.Ctx(ctx).Err(err).Stringer("room_id", invite.ID).Msg("Failed to marshal invite push")
			continue
		} else if len(raw) > 1500 {
			zerolog.Ctx(ctx).Error().Stringer("room_id", invite.ID).Msg("Invite push too long")
			continue
		}
		push.RawInvites = append(push.RawInvites, raw)
		invitePushed.Store(invite.ID, struct{}{})
	}
}

// sendForkInvitePushes sends each invite as a push of its own: they're rare, and keeping them out
// of PushNotification.Split keeps that upstream code untouched.
func (gmx *Gomuks) sendForkInvitePushes(ctx context.Context, pushRegs []*database.PushRegistration, push *PushNotification) {
	for _, raw := range push.RawInvites {
		gmx.SendPushNotification(ctx, pushRegs, &PushNotification{
			RawInvites:   []json.RawMessage{raw},
			ImageAuth:    push.ImageAuth,
			HasImportant: true,
		})
	}
}

// inviteRoom is just enough of a room to run the push rules on our invite.
type inviteRoom struct {
	ownName string
	members int
}

func (r *inviteRoom) GetOwnDisplayname() string { return r.ownName }
func (r *inviteRoom) GetMemberCount() int       { return r.members }

type inviteMemberContent struct {
	Membership  event.Membership    `json:"membership"`
	Displayname string              `json:"displayname"`
	AvatarURL   id.ContentURIString `json:"avatar_url"`
	IsDirect    bool                `json:"is_direct"`
	Reason      string              `json:"reason"`
}

func inviteContent(evt *event.Event, into any) bool {
	raw := evt.Content.VeryRaw
	if len(raw) == 0 {
		var err error
		raw, err = json.Marshal(&evt.Content)
		if err != nil {
			return false
		}
	}
	return json.Unmarshal(raw, into) == nil
}

func truncateForPush(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func (gmx *Gomuks) formatInvitePush(ctx context.Context, invite *database.InvitedRoom) *PushInvite {
	self := gmx.Client.Account.UserID
	var ownInvite *event.Event
	var own inviteMemberContent
	var roomName, alias string
	var roomAvatar id.ContentURIString
	var isSpace bool
	members := make(map[id.UserID]inviteMemberContent)
	for _, evt := range invite.InviteState {
		switch evt.Type {
		case event.StateMember:
			var content inviteMemberContent
			if !inviteContent(evt, &content) {
				continue
			}
			userID := id.UserID(evt.GetStateKey())
			members[userID] = content
			if userID == self && content.Membership == event.MembershipInvite {
				ownInvite, own = evt, content
			}
		case event.StateRoomName:
			var content event.RoomNameEventContent
			if inviteContent(evt, &content) {
				roomName = content.Name
			}
		case event.StateCanonicalAlias:
			var content event.CanonicalAliasEventContent
			if inviteContent(evt, &content) {
				alias = content.Alias.String()
			}
		case event.StateRoomAvatar:
			var content event.RoomAvatarEventContent
			if inviteContent(evt, &content) {
				roomAvatar = content.URL
			}
		case event.StateCreate:
			var content event.CreateEventContent
			if inviteContent(evt, &content) {
				isSpace = content.Type == event.RoomTypeSpace
			}
		}
	}
	if ownInvite == nil || ownInvite.Sender == "" {
		// Not an invite we can tell anything about (or one we already answered).
		return nil
	}
	log := zerolog.Ctx(ctx).With().Stringer("room_id", invite.ID).Logger()
	if rules := gmx.Client.PushRules.Load(); rules != nil {
		evt := *ownInvite
		evt.RoomID = invite.ID
		joined := 0
		for _, member := range members {
			if member.Membership == event.MembershipJoin {
				joined++
			}
		}
		room := &inviteRoom{ownName: own.Displayname, members: joined}
		if !rules.GetActions(room, &evt).Should().Notify {
			log.Debug().Msg("Push rules say not to notify for invite")
			return nil
		}
	}

	inviterID := ownInvite.Sender
	inviter := NotificationUser{ID: inviterID, Name: inviterID.Localpart()}
	if member, ok := members[inviterID]; ok && (member.Displayname != "" || member.AvatarURL != "") {
		inviter.Name = cmp.Or(member.Displayname, inviter.Name)
		inviter.Avatar = getAvatarLinkForNotification(member.Displayname, inviterID.String(), member.AvatarURL)
	} else {
		profileCtx, cancel := context.WithTimeout(ctx, inviteProfileTimeout)
		profile, err := gmx.Client.Client.GetProfile(profileCtx, inviterID)
		cancel()
		if err != nil {
			log.Debug().Err(err).Stringer("inviter", inviterID).Msg("Failed to get inviter profile for push")
		} else if profile != nil {
			inviter.Name = cmp.Or(profile.DisplayName, inviter.Name)
			inviter.Avatar = getAvatarLinkForNotification(profile.DisplayName, inviterID.String(), profile.AvatarURL.CUString())
		}
	}
	inviter.Name = truncateForPush(inviter.Name, 50)

	name := truncateForPush(cmp.Or(roomName, alias), 50)
	var avatar string
	if roomAvatar != "" {
		avatar = getAvatarLinkForNotification(name, invite.ID.String(), roomAvatar)
	}
	ownUser := NotificationUser{ID: self, Name: cmp.Or(own.Displayname, self.Localpart())}
	ownUser.Name = truncateForPush(ownUser.Name, 50)
	if own.AvatarURL != "" {
		ownUser.Avatar = getAvatarLinkForNotification(own.Displayname, self.String(), own.AvatarURL)
	}
	timestamp := invite.CreatedAt
	if timestamp.IsZero() {
		timestamp = jsontime.UM(time.Now())
	}
	return &PushInvite{
		Timestamp:  timestamp,
		RoomID:     invite.ID,
		RoomName:   name,
		RoomAvatar: avatar,
		IsDM:       own.IsDirect,
		IsSpace:    isSpace,
		Inviter:    inviter,
		Self:       ownUser,
		Reason:     truncateForPush(own.Reason, maxInviteReasonLength),
	}
}
