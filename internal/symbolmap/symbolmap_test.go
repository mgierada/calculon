package symbolmap

import "testing"

func TestToFinimpulse(t *testing.T) {
	cases := []struct {
		symbol string
		want   string
		ok     bool
	}{
		{"XTB.PL", "XTB.WA", true},
		{"ETFBS80TR.PL", "ETFBS80TR.WA", true},
		{"AMT.US", "AMT", true},
		{"MXFS.UK", "MXFS.L", true},
		{"SAP.DE", "SAP.DE", true},
		{"US500", "", false},
		{"ABC.XX", "", false},
		{".PL", "", false},
	}
	for _, c := range cases {
		got, ok := ToFinimpulse(c.symbol)
		if got != c.want || ok != c.ok {
			t.Errorf("ToFinimpulse(%q) = %q, %v; want %q, %v", c.symbol, got, ok, c.want, c.ok)
		}
	}
}
