// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package database

import (
	"context"
)

const getTotalUnreadHighlightsQuery = `SELECT COALESCE(SUM(unread_highlights), 0) FROM room`

func (rq *RoomQuery) GetTotalUnreadHighlights(ctx context.Context) (count int, err error) {
	err = rq.GetDB().QueryRow(ctx, getTotalUnreadHighlightsQuery).Scan(&count)
	return
}
