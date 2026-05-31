package room

import "testing"

func TestValidProgress(t *testing.T) {
	const total = 10
	cases := []struct {
		name       string
		prev, idx  int
		wantValid  bool
	}{
		{"advance by one", 3, 4, true},
		{"advance by several", 3, 7, true},
		{"reach the end", 9, 10, true},
		{"no progress (replay)", 4, 4, false},
		{"backwards", 5, 2, false},
		{"overshoot past total", 9, 11, false},
		{"jump straight to total from zero", 0, 10, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := validProgress(c.prev, c.idx, total); got != c.wantValid {
				t.Errorf("validProgress(%d, %d, %d) = %v, want %v",
					c.prev, c.idx, total, got, c.wantValid)
			}
		})
	}
}

func TestSanitizeName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"  Alice  ", "Alice"},
		{"", "名無し"},
		{"   ", "名無し"},
		{"line\nbreak", "linebreak"},
		// 27 runes in, trimmed to 24 (ながい×9 -> ながい×8)
		{"ながいながいながいながいながいながいながいながいながい", "ながいながいながいながいながいながいながいながい"},
	}
	for _, c := range cases {
		if got := sanitizeName(c.in); got != c.want {
			t.Errorf("sanitizeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
