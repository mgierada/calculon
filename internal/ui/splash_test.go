package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
)

func splashApp(t *testing.T) *App {
	t.Helper()

	dashboards := []Dashboard{{Title: "Overview", Build: func(*portfolio.Report) Component {
		return &stub{name: "overview body"}
	}}}
	app := NewApp(dashboards, func(*model.AccountKey) (portfolio.Report, error) { return testReport(), nil }).
		WithSplash(Splash{User: "maciej", MinDuration: time.Second})
	app.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	return app
}

func showsSplash(app *App) bool {
	return strings.Contains(app.View().Content, "Logging in as")
}

func TestSplashShowsLogoAndUser(t *testing.T) {
	view := splashApp(t).View()

	if !view.AltScreen {
		t.Error("splash is not fullscreen")
	}
	for _, want := range []string{"Logging in as", "maciej", "▄"} {
		if !strings.Contains(view.Content, want) {
			t.Errorf("splash is missing %q", want)
		}
	}
}

// A fast load still waits for the minimum duration.
func TestSplashWaitsForMinimumDuration(t *testing.T) {
	app := splashApp(t)

	app.Update(reportMsg{report: testReport()})
	if !showsSplash(app) {
		t.Fatal("splash left before its minimum duration")
	}
	app.Update(splashElapsedMsg{})
	if showsSplash(app) || !strings.Contains(app.View().Content, "overview body") {
		t.Errorf("dashboard not shown after load and timeout:\n%s", app.View().Content)
	}
}

// A slow load keeps the splash up past the minimum duration.
func TestSplashWaitsForLoading(t *testing.T) {
	app := splashApp(t)

	app.Update(splashElapsedMsg{})
	if !showsSplash(app) {
		t.Fatal("splash left before the report loaded")
	}
	app.Update(reportMsg{report: testReport()})
	if showsSplash(app) {
		t.Error("splash still up after load and timeout")
	}
}

func TestSplashInitStartsLoadAndTimer(t *testing.T) {
	if cmd := splashApp(t).Init(); cmd == nil {
		t.Fatal("Init returned no command")
	}
}

func TestSplashOnlyAllowsQuitting(t *testing.T) {
	app := splashApp(t)

	if _, cmd := app.Update(press("?")); cmd != nil || app.modal != nil {
		t.Error("help opened over the splash")
	}
	if _, cmd := app.Update(press("q")); cmd == nil {
		t.Error("q did not quit from the splash")
	}
}

func TestSplashPicksTheLargestLogoThatFits(t *testing.T) {
	bigWidth := lipgloss.Width(renderLogo(logo))
	smallWidth := lipgloss.Width(renderLogo(logoSmall))
	tinyWidth := lipgloss.Width(renderLogo(logoTiny))
	if !(tinyWidth < smallWidth && smallWidth < bigWidth) {
		t.Fatalf("logo widths %d, %d, %d are not strictly decreasing", bigWidth, smallWidth, tinyWidth)
	}

	tests := []struct {
		name          string
		width, height int
		want          string
	}{
		{"wide", bigWidth + 10, 40, renderLogo(logo)},
		{"medium", smallWidth + 5, 40, renderLogo(logoSmall)},
		{"small", smallWidth - 1, 40, renderLogo(logoTiny)},
		{"narrow", tinyWidth - 1, 40, wordmark},
		{"short", bigWidth + 10, 6, wordmark},
	}
	for _, test := range tests {
		app := splashApp(t)
		app.Update(tea.WindowSizeMsg{Width: test.width, Height: test.height})
		view := app.View().Content
		if !strings.Contains(view, strings.SplitN(test.want, "\n", 2)[0]) {
			t.Errorf("%s terminal (%dx%d) does not show the expected logo:\n%s",
				test.name, test.width, test.height, view)
		}
		if w := lipgloss.Width(view); w > test.width {
			t.Errorf("%s terminal: splash is %d wide, more than %d", test.name, w, test.width)
		}
	}
}

func TestPinPaletteUsesVGAColours(t *testing.T) {
	tests := map[string]string{
		"\x1b[0;95m":    "\x1b[0;38;2;255;85;255m",
		"\x1b[0;35;41m": "\x1b[0;38;2;170;0;170;48;2;170;0;0m",
		"\x1b[0;30;41m": "\x1b[0;38;2;0;0;0;48;2;170;0;0m",
		"\x1b[0m":       "\x1b[0m",
		"\x1b[1;4m":     "\x1b[1;4m",
	}
	for in, want := range tests {
		if got := pinPalette(in + "█"); got != want+"█" {
			t.Errorf("pinPalette(%q) = %q, want %q", in, got, want+"█")
		}
	}
}

// The splash shows the logo's own colours, pinned to VGA, and nothing else.
func TestSplashKeepsLogoColours(t *testing.T) {
	view := splashApp(t).View().Content
	if !strings.Contains(view, pinPalette(strings.SplitN(logo, "\n", 2)[0])) {
		t.Error("splash does not contain the logo's first line with its colours")
	}
}
