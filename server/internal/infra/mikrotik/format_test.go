package mikrotik

import "testing"

func TestFormatLimitUptime(t *testing.T) {
	cases := []struct {
		secs int64
		want string
	}{
		{0, "00:00:00"},
		{-5, "00:00:00"},
		{60, "00:01:00"},
		{3600, "01:00:00"},
		{43200, "12:00:00"},     // contoh tech_spec
		{604800, "7d 00:00:00"}, // contoh tech_spec
		{90000, "1d 01:00:00"},  // 25 jam
		{86400 + 3661, "1d 01:01:01"},
		{30 * 86400, "30d 00:00:00"},
	}
	for _, c := range cases {
		if got := FormatLimitUptime(c.secs); got != c.want {
			t.Errorf("FormatLimitUptime(%d) = %q, want %q", c.secs, got, c.want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"3600", 3600, true},
		{"1h", 3600, true},
		{"5m", 300, true},
		{"12:00:00", 43200, true},
		{"7d 00:00:00", 604800, true},
		{"1d 01:00:00", 90000, true},
		{"1w2d", 777600, true},
		{"", 0, false},
		{"bogus", 0, false},
	}
	for _, c := range cases {
		got, ok := parseDuration(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseDuration(%q) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
