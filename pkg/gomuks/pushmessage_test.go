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
