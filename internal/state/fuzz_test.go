package state

import "testing"

func FuzzParse(f *testing.F) {
	f.Add("stateDiagram-v2\n[*] --> A\nA --> [*]\nstate A\n")
	f.Add("stateDiagram-v2\n[*] --> A\nA --> B : retry\nstate A\nstate B\npolicy A --> B : retry :: retry \"attempt below 3\"\n")
	f.Add("stateDiagram-v2\n")
	f.Fuzz(func(t *testing.T, source string) {
		d, err := Parse(source, DefaultLimits())
		if err != nil && d != nil {
			t.Fatal("parse error returned a partial diagram")
		}
		if err == nil && (len(d.States) == 0 || len(d.Transitions) == 0) {
			t.Fatal("successful parse broke invariants")
		}
	})
}
