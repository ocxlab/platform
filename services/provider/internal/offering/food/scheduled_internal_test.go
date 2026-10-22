package food

import "testing"

func TestMinuteParserDefensiveBoundaries(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want int
	}{{"", -1}, {"00-00", -1}, {"aa:00", -1}, {"00:bb", -1}, {"-1:00", -1}, {"00:-1", -1}, {"25:00", -1}, {"00:60", -1}, {"24:01", -1}, {"00:00", 0}, {"00:59", 59}, {"01:00", 60}, {"23:59", 1439}, {"24:00", 1440}} {
		if got := minute(test.raw); got != test.want {
			t.Errorf("minute(%q)=%d want %d", test.raw, got, test.want)
		}
	}
	if validWindows([]Window{{Start: "24:00", End: "24:00"}}) || validWindows([]Window{{Start: "00:00", End: "-1:00"}}) || validWindows([]Window{{Start: "01:00", End: "01:00"}}) {
		t.Fatal("invalid window accepted")
	}
}
