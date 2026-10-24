package mikrotik

import "testing"

func TestParseAllowRemoteRequests(t *testing.T) {
	ya := []string{"yes", "YES", " true ", "True", "TRUE", "1"}
	for _, v := range ya {
		if !parseAllowRemoteRequests(v) {
			t.Fatalf("%q seharusnya true", v)
		}
	}
	tidak := []string{"no", "NO", "false", "False", "0", "", "maybe"}
	for _, v := range tidak {
		if parseAllowRemoteRequests(v) {
			t.Fatalf("%q seharusnya false", v)
		}
	}
}
