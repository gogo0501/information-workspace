// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import "testing"

func TestSnippetNormalizesAndTruncatesContent(t *testing.T) {
	got := snippet("  one\n\t two three four ", 11)
	if got != "one two thr..." {
		t.Fatalf("unexpected snippet: %q", got)
	}
}

func TestSnippetKeepsShortContent(t *testing.T) {
	if got := snippet("one two", 20); got != "one two" {
		t.Fatalf("unexpected snippet: %q", got)
	}
}
