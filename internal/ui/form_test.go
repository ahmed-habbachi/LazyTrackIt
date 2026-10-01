package ui

import "testing"

func TestTimeFieldTypeDigitHour(t *testing.T) {
	cases := []struct {
		name        string
		digits      []int
		wantHour    int
		wantAdvance bool // segment should have moved on to minutes
	}{
		{"two digits 21", []int{2, 1}, 21, true},
		{"two digits 05", []int{0, 5}, 5, true},
		{"single digit 9 forces advance (90 would overflow)", []int{9}, 9, true},
		{"single digit 1 waits for a second", []int{1}, 1, false},
		{"second digit too big restarts and forces advance", []int{2, 9}, 9, true},
		{"two digits 23 (max valid hour)", []int{2, 3}, 23, true},
		{"second digit overflow clamps combined value", []int{9, 9}, 9, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tf := newTimeField(0)
			for _, d := range c.digits {
				tf.typeDigit(d)
			}
			if tf.hour() != c.wantHour {
				t.Errorf("hour = %d, want %d", tf.hour(), c.wantHour)
			}
			gotAdvance := tf.segment == 1
			if gotAdvance != c.wantAdvance {
				t.Errorf("segment advanced = %v, want %v", gotAdvance, c.wantAdvance)
			}
		})
	}
}

func TestTimeFieldTypeDigitMinute(t *testing.T) {
	tf := newTimeField(0)
	tf.typeDigit(1)
	tf.typeDigit(0) // hour = 10, auto-advanced to minute segment
	if tf.segment != 1 {
		t.Fatalf("expected to have advanced to minute segment, segment=%d", tf.segment)
	}
	tf.typeDigit(3)
	tf.typeDigit(5)
	if got := tf.minute(); got != 35 {
		t.Errorf("minute = %d, want 35", got)
	}
	if got, want := tf.minutes, 10*60+35; got != want {
		t.Errorf("minutes = %d, want %d", got, want)
	}
}

func TestTimeFieldRetypeReplacesRatherThanAppends(t *testing.T) {
	tf := newTimeField(9*60 + 30)
	tf.segment = 0
	tf.entered = 0 // as if freshly (re)focused

	tf.typeDigit(2)
	if got := tf.hour(); got != 2 {
		t.Fatalf("after first digit, hour = %d, want 2 (old value should be fully replaced)", got)
	}
}

func TestTimeFieldStepWraps(t *testing.T) {
	tf := newTimeField(0) // 00:00
	tf.segment = 0
	tf.step(-1)
	if tf.hour() != 23 {
		t.Errorf("hour wrap down = %d, want 23", tf.hour())
	}

	tf = newTimeField(23 * 60)
	tf.segment = 0
	tf.step(1)
	if tf.hour() != 0 {
		t.Errorf("hour wrap up = %d, want 0", tf.hour())
	}

	tf = newTimeField(59) // 00:59
	tf.segment = 1
	tf.step(1)
	if tf.minute() != 0 {
		t.Errorf("minute wrap up = %d, want 0", tf.minute())
	}
}
