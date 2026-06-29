package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// keyStub records the keys it was sent.
type keyStub struct {
	stub
	keys []string
}

func (k *keyStub) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		k.keys = append(k.keys, key.String())
	}
	return nil
}

func press(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: key, Code: rune(key[0])})
}

func TestAppFocusesFirstComponent(t *testing.T) {
	first, second := &stub{name: "first"}, &stub{name: "second"}
	NewApp(Stack(first, second))

	if !first.focused {
		t.Error("first component is not focused")
	}
	if second.focused {
		t.Error("second component is focused")
	}
}

func TestAppRoutesKeysToFocusedComponentOnly(t *testing.T) {
	first, second := &keyStub{stub: stub{name: "first"}}, &keyStub{stub: stub{name: "second"}}
	app := NewApp(Stack(first, second))

	app.Update(press("j"))

	if len(first.keys) != 1 || first.keys[0] != "j" {
		t.Errorf("focused component received %v, want [j]", first.keys)
	}
	if len(second.keys) != 0 {
		t.Errorf("unfocused component received %v, want nothing", second.keys)
	}
}

func TestAppCyclesFocusOnTab(t *testing.T) {
	first, second := &keyStub{stub: stub{name: "first"}}, &keyStub{stub: stub{name: "second"}}
	app := NewApp(Stack(first, second))

	app.Update(tea.KeyPressMsg(tea.Key{Text: focusKey, Code: tea.KeyTab}))
	if first.focused || !second.focused {
		t.Fatal("tab did not move focus to the second component")
	}

	app.Update(press("k"))
	if len(second.keys) != 1 || second.keys[0] != "k" {
		t.Errorf("second component received %v, want [k]", second.keys)
	}
	if len(first.keys) != 0 {
		t.Errorf("first component received %v after losing focus, want nothing", first.keys)
	}

	app.Update(tea.KeyPressMsg(tea.Key{Text: focusKey, Code: tea.KeyTab}))
	if !first.focused || second.focused {
		t.Error("tab did not wrap focus back to the first component")
	}
}

func TestAppQuitsOnQuitKeys(t *testing.T) {
	for key := range quitKeys {
		app := NewApp(Single(&stub{name: "table"}))
		_, cmd := app.Update(tea.KeyPressMsg(tea.Key{Text: key, Code: rune(key[0])}))
		if cmd == nil {
			t.Errorf("key %q returned no command, want quit", key)
		}
	}
}

func TestAppResizesGrid(t *testing.T) {
	table := &stub{name: "table"}
	app := NewApp(Single(table))

	app.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	if got, want := table.box(), "100x40"; got != want {
		t.Errorf("table box = %s, want %s", got, want)
	}
}

func TestAppStartsAtADefaultSize(t *testing.T) {
	table := &stub{name: "table"}
	NewApp(Single(table))

	if got, want := table.box(), "80x24"; got != want {
		t.Errorf("table box before any resize = %s, want %s", got, want)
	}
}

// A terminal that cannot report its size must not collapse the layout.
func TestAppIgnoresZeroSize(t *testing.T) {
	table := &stub{name: "table"}
	app := NewApp(Single(table))
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	app.Update(tea.WindowSizeMsg{Width: 0, Height: 0})

	if got, want := table.box(), "100x40"; got != want {
		t.Errorf("table box after a zero resize = %s, want %s", got, want)
	}
}

func TestAppViewIsFullscreen(t *testing.T) {
	app := NewApp(Single(&stub{name: "table"}))

	view := app.View()

	if !view.AltScreen {
		t.Error("view does not request the alternate screen")
	}
	if view.Content != "table" {
		t.Errorf("view content = %q, want %q", view.Content, "table")
	}
}
