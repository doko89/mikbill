package sqlite

import "time"

// parseTime membaca timestamp SQLite (strftime %Y-%m-%dT%H:%M:%fZ).
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// formatTime menulis timestamp UTC untuk kolom TEXT.
func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

// formatTimeNow menulis timestamp sekarang dalam UTC.
func formatTimeNow() string {
	return formatTime(time.Now())
}
