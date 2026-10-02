package ui

import (
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// binding documents one key for the help screen.
type binding struct {
	keys        string
	description string
}

// bindingGroups are every key the app and its widgets react to, by where
// they apply. Adding a key to a widget means adding it here too.
var bindingGroups = []struct {
	name     string
	bindings []binding
}{
	{"General", []binding{
		{"1 … 9", "switch to dashboard"},
		{"[ / ]", "previous / next dashboard"},
		{"a", "choose account (all or one)"},
		{"r", "reload data"},
		{"?", "show this help"},
		{"q / ctrl+c", "quit"},
	}},
	{"Focus", []binding{
		{"tab", "focus next widget"},
		{"shift+tab", "focus previous widget"},
		{"esc", "close details"},
	}},
	{"Tables", []binding{
		{"j / down", "next row"},
		{"k / up", "previous row"},
		{"l / right", "next page"},
		{"h / left", "previous page"},
		{"g / home", "first page"},
		{"G / end", "last page"},
		{"enter", "open details of the row"},
		{"/", "filter rows"},
		{"s", "sort by next column"},
		{"S", "reverse sort direction"},
		{"enter (filter)", "keep filter"},
		{"esc (filter)", "clear filter"},
		{"shift+→ / shift+←", "scroll columns"},
	}},
	{"Overview", []binding{
		{"g", "allocation: change grouping"},
		{"m", "returns: cycle TWR / XIRR / CAGR"},
	}},
}

// helpItems lists the bindings for the help picker.
func helpItems() []widgets.PickerItem {
	var items []widgets.PickerItem
	for _, group := range bindingGroups {
		for _, b := range group.bindings {
			items = append(items, widgets.PickerItem{Key: b.keys, Label: b.description, Group: group.name})
		}
	}
	return items
}
