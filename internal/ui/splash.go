package ui

import (
	_ "embed"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

//go:embed assets/logo.txt
var logo string

//go:embed assets/logo_small.txt
var logoSmall string

//go:embed assets/logo_tiny.txt
var logoTiny string

// logos are tried largest first; the splash shows the first that fits.
var logos = []string{logo, logoSmall, logoTiny}

// wordmark stands in when no logo fits the terminal.
const wordmark = "termfolio.ssh"

// messageHeight is the gap and the line of text under the logo.
const messageHeight = 3

// DefaultSplashDuration is the least time the splash stays up.
const DefaultSplashDuration = 3000 * time.Millisecond

// ansiReset ends whatever colour a logo line left active.
const ansiReset = "\x1b[0m"

// sgrSequence matches one SGR escape, e.g. "\x1b[0;95;41m".
var sgrSequence = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// vgaPalette is the classic VGA rendering of the 16 basic ANSI colours, the
// palette the logo was drawn in: normal colours 0..7, then bright 8..15.
// Terminal themes redraw these colours however they like, so the logo pins
// them to exact RGB values instead.
var vgaPalette = [16][3]uint8{
	{0x00, 0x00, 0x00}, {0xAA, 0x00, 0x00}, {0x00, 0xAA, 0x00}, {0xAA, 0x55, 0x00},
	{0x00, 0x00, 0xAA}, {0xAA, 0x00, 0xAA}, {0x00, 0xAA, 0xAA}, {0xAA, 0xAA, 0xAA},
	{0x55, 0x55, 0x55}, {0xFF, 0x55, 0x55}, {0x55, 0xFF, 0x55}, {0xFF, 0xFF, 0x55},
	{0x55, 0x55, 0xFF}, {0xFF, 0x55, 0xFF}, {0x55, 0xFF, 0xFF}, {0xFF, 0xFF, 0xFF},
}

var (
	splashMessageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	splashUserStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("116"))
	wordmarkStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
)

// Splash is the fullscreen welcome shown while the app gets ready. It stays up
// until the first report has loaded and at least MinDuration has passed, so
// any work done before the dashboards can be shown (the loader, and whatever
// it grows to call) happens behind it.
type Splash struct {
	// User is who is logging in, shown under the logo.
	User string
	// MinDuration keeps the splash up even when loading is instant.
	MinDuration time.Duration
}

// splashElapsedMsg says the splash has been up for its minimum duration.
type splashElapsedMsg struct{}

// WithSplash shows the splash before the dashboards.
func (a *App) WithSplash(splash Splash) *App {
	a.splash = &splashState{Splash: splash}
	return a
}

// splashState tracks the two things the splash waits for.
type splashState struct {
	Splash
	elapsed bool
	loaded  bool
}

func (s *splashState) done() bool {
	return s.elapsed && s.loaded
}

// tick fires once the minimum duration has passed.
func (s *splashState) tick() tea.Cmd {
	return tea.Tick(s.MinDuration, func(time.Time) tea.Msg { return splashElapsedMsg{} })
}

// view renders the logo and message centred on the whole screen, using the
// largest logo that fits and falling back to a plain wordmark.
func (s *splashState) view(width, height int) string {
	welcome := fmt.Sprintf("Logging in as %s...", splashUserStyle.Render(s.User))
	message := splashMessageStyle.Render(welcome)
	if s.User == "" {
		message = splashMessageStyle.Render("Loading…")
	}
	content := lipgloss.JoinVertical(lipgloss.Center, fittingLogo(width, height), "", "", message)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

// fittingLogo is the largest logo that fits the screen with the message under
// it, or the wordmark when none does.
func fittingLogo(width, height int) string {
	for _, art := range logos {
		rendered := renderLogo(art)
		if lipgloss.Width(rendered) <= width && lipgloss.Height(rendered)+messageHeight <= height {
			return rendered
		}
	}
	return wordmarkStyle.Render(wordmark)
}

// renderLogo returns the logo as drawn. The file carries its own ANSI colours;
// they are only pinned to the VGA palette, and each line is reset so its
// colour cannot bleed into the padding or the message below.
func renderLogo(art string) string {
	lines := strings.Split(strings.TrimRight(art, "\n"), "\n")
	for i, line := range lines {
		lines[i] = pinPalette(line) + ansiReset
	}
	return strings.Join(lines, "\n")
}

// pinPalette rewrites basic ANSI colour codes as the equivalent VGA RGB
// colours, leaving every other attribute as it is.
func pinPalette(s string) string {
	return sgrSequence.ReplaceAllStringFunc(s, func(seq string) string {
		params := strings.Split(sgrSequence.FindStringSubmatch(seq)[1], ";")
		for i, param := range params {
			code, err := strconv.Atoi(param)
			if err != nil {
				continue
			}
			if rgb, ok := basicColour(code); ok {
				params[i] = rgb
			}
		}
		return "\x1b[" + strings.Join(params, ";") + "m"
	})
}

// basicColour maps a basic foreground or background code to its truecolor
// form, e.g. 95 (bright magenta) to "38;2;255;85;255".
func basicColour(code int) (string, bool) {
	var layer, index int
	switch {
	case code >= 30 && code <= 37:
		layer, index = 38, code-30
	case code >= 90 && code <= 97:
		layer, index = 38, code-90+8
	case code >= 40 && code <= 47:
		layer, index = 48, code-40
	case code >= 100 && code <= 107:
		layer, index = 48, code-100+8
	default:
		return "", false
	}
	c := vgaPalette[index]
	return fmt.Sprintf("%d;2;%d;%d;%d", layer, c[0], c[1], c[2]), true
}
