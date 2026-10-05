package widgets

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// listChooseKey picks the highlighted item.
const listChooseKey = "enter"

var (
	listTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).Padding(0, 1)
	listSelectedTitle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("#25A065")).Foreground(lipgloss.Color("#04B575")).
				Padding(0, 0, 0, 1)
	listSelectedDesc = listSelectedTitle.Foreground(lipgloss.Color("#3C9D6E"))
)

// ListItem is one choice of a ListPicker. Value is handed back when chosen.
type ListItem struct {
	Name   string
	Detail string
	Value  any
}

// Title implements list.DefaultItem.
func (i ListItem) Title() string { return i.Name }

// Description implements list.DefaultItem.
func (i ListItem) Description() string { return i.Detail }

// FilterValue implements list.Item, filtering on the name and detail.
func (i ListItem) FilterValue() string { return i.Name + " " + i.Detail }

// ListPicker is a full-screen, filterable list in the style of bubbletea's
// list-fancy example: two-line items, / to fuzzy-filter, enter to choose.
type ListPicker struct {
	list    list.Model
	choose  func(ListItem) tea.Cmd
	width   int
	height  int
	focused bool
}

// NewListPicker lists items; choose runs on enter with the highlighted one.
func NewListPicker(title string, items []ListItem, choose func(ListItem) tea.Cmd) *ListPicker {
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = listSelectedTitle
	delegate.Styles.SelectedDesc = listSelectedDesc
	l := list.New(listItems, delegate, 0, 0)
	l.Title = title
	l.Styles.Title = listTitleStyle
	l.SetStatusBarItemName("holding", "holdings")
	// The app owns quitting.
	l.KeyMap.Quit.SetEnabled(false)
	l.KeyMap.ForceQuit.SetEnabled(false)
	return &ListPicker{list: l, choose: choose}
}

// Init implements ui.Component.
func (p *ListPicker) Init() tea.Cmd { return nil }

// Update implements ui.Component: enter chooses, everything else moves or
// filters the list.
func (p *ListPicker) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == listChooseKey && !p.list.SettingFilter() {
		if item, ok := p.list.SelectedItem().(ListItem); ok {
			return p.choose(item)
		}
		return nil
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return cmd
}

// CapturingInput implements ui.InputCapturer while the filter is typed.
func (p *ListPicker) CapturingInput() bool {
	return p.list.SettingFilter()
}

// SetSize implements ui.Component.
func (p *ListPicker) SetSize(width, height int) {
	p.width, p.height = width, height
	innerWidth, innerHeight := innerSize(width, height)
	p.list.SetSize(innerWidth, innerHeight)
}

// SetFocused implements ui.Component.
func (p *ListPicker) SetFocused(focused bool) { p.focused = focused }

// View implements ui.Component.
func (p *ListPicker) View() string {
	return frame(p.width, p.height, p.focused).Render(p.list.View())
}
