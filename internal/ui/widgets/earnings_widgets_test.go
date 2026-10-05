package widgets

import (
	"regexp"
	"strings"
	"testing"
)

func plainText(s string) string {
	return regexp.MustCompile(`\x1b\[[0-9;:]*[a-zA-Z]`).ReplaceAllString(s, "")
}

func f(v float64) *float64 { return &v }

// The target price can sit outside the analysts' range; the track must still
// place it, at the end.
func TestRangeSliderSpansEveryMark(t *testing.T) {
	slider := NewRangeSlider("t", LowMark(f(1060)), HighMark(f(1380)), AverageMark(f(1234.19)), TargetMark(f(1011.58)))

	track := []rune(plainText(slider.Track(41)))
	if len(track) != 41 || track[0] != '◆' || track[40] != '┤' {
		t.Errorf("track = %q, want the target first and the high last", string(track))
	}
	// (1060 - 1011.58) / (1380 - 1011.58) of 40 cells rounds to 5.
	if track[5] != '├' {
		t.Errorf("track = %q, want the low at 5", string(track))
	}
	slider.SetSize(100, 6)
	if view := plainText(slider.View()); !strings.Contains(view, "◆ target 1011.58") ||
		!strings.Contains(view, "● avg 1234.19") {
		t.Errorf("labels missing:\n%s", view)
	}
}

func TestRangeSliderLeavesOutUnknownMarks(t *testing.T) {
	track := plainText(NewRangeSlider("t", LowMark(nil), AverageMark(f(5))).Track(10))
	if strings.Contains(track, "├") || !strings.Contains(track, "●") {
		t.Errorf("track = %q, want only the known mark", track)
	}
}

func TestMatrixShowsNewestAndScrollsToOlder(t *testing.T) {
	periods := []string{"Q1 '25", "Q2 '25", "Q3 '25", "Q4 '25", "Q1 '26"}
	revenue := []*float64{f(1), f(2), nil, f(4), f(-5)}
	format := func(v float64) string { return strings.Repeat("9", 8) }
	m := NewMatrix("revenue", "data in mln USD", periods, MatrixRow{Label: "Revenue", Values: revenue, Format: format})
	m.SetSize(60, m.Height())

	view := plainText(m.View())
	if !strings.Contains(view, "Q1 '26") || strings.Contains(view, "Q1 '25") || !strings.Contains(view, "data in mln USD") {
		t.Errorf("narrow matrix should show the newest periods:\n%s", view)
	}
	for range 4 {
		m.Scroll("left")
	}
	if view := plainText(m.View()); !strings.Contains(view, "Q1 '25") || strings.Contains(view, "Q1 '26") {
		t.Errorf("scrolled matrix should show the oldest periods:\n%s", view)
	}
	if got := plainText(sparkline(revenue)); got != "▂▄ ▇█" {
		t.Errorf("sparkline = %q", got)
	}
}
