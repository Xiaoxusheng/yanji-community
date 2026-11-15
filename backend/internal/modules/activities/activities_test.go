package activities

import "testing"

func TestItoa(t *testing.T) {
	cases := map[int64]string{0: "0", 7: "7", -3: "-3", 123456: "123456"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Fatalf("itoa(%d) = %s, want %s", in, got, want)
		}
	}
}
