package generated

import "testing"

func TestDouble(t *testing.T) {
	if got := Double(2); got != 4 {
		t.Errorf("Double(2) = %d, want 4", got)
	}
}
