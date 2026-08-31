package textcell

import "testing"

func TestWidth(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{name: "ascii", text: "API", want: 3},
		{name: "hangul", text: "수파", want: 4},
		{name: "cjk", text: "事件", want: 4},
		{name: "combining", text: "e\u0301", want: 1},
		{name: "fullwidth", text: "Ａ", want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Width(tt.text)
			if err != nil {
				t.Fatalf("Width() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Width() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWidthRejectsTerminalControlsAndBidi(t *testing.T) {
	for _, input := range []string{"safe\x1b[31m", "safe\x00", "left\u202eright", "x\u2066y", "\ufeffBOM", "x\u200dy", "\u0301leading"} {
		if _, err := Width(input); err == nil {
			t.Fatalf("Width(%q) expected error", input)
		}
	}
}

func TestWidthRejectsCombiningMarkFlood(t *testing.T) {
	if _, err := Width("a\u0301\u0301\u0301\u0301\u0301\u0301\u0301\u0301\u0301"); err == nil {
		t.Fatal("expected combining mark limit error")
	}
}
