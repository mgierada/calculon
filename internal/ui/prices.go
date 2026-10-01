package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/mgierada/calculon/internal/marketdata"
)

// pricesMsg delivers a market data poll that stored new prices.
type pricesMsg marketdata.Update

// prices tracks the market data feed the dashboards reload on.
type prices struct {
	updates <-chan marketdata.Update
	last    *marketdata.Update
}

// WithPrices reloads the dashboards whenever updates delivers new prices.
func (a *App) WithPrices(updates <-chan marketdata.Update) *App {
	a.prices = &prices{updates: updates}
	return a
}

// waitPrices waits off the UI loop for the next update, giving up quietly
// once the feed closes.
func (a *App) waitPrices() tea.Cmd {
	if a.prices == nil {
		return nil
	}
	updates := a.prices.updates
	return func() tea.Msg {
		update, ok := <-updates
		if !ok {
			return nil
		}
		return pricesMsg(update)
	}
}

// handlePrices remembers the poll for the footer, reloads keeping the view and
// waits again. While a component takes text the reload waits until it is done.
func (a *App) handlePrices(msg pricesMsg) tea.Cmd {
	update := marketdata.Update(msg)
	a.prices.last = &update
	if capturer, ok := focusedOf(a.current()).(InputCapturer); ok && capturer.CapturingInput() {
		a.staleView = true
		return a.waitPrices()
	}
	return tea.Batch(a.reload(true), a.waitPrices())
}

// pricesStatus describes the last poll for the footer, warning about symbols
// it failed to price. Both are empty before the first poll.
func (a *App) pricesStatus() (info, warning string) {
	if a.prices == nil || a.prices.last == nil {
		return "", ""
	}
	last := a.prices.last
	info = "prices " + last.At.Local().Format("15:04")
	if n := len(last.Failed); n > 0 {
		warning = fmt.Sprintf("%d prices failed", n)
	}
	return info, warning
}
