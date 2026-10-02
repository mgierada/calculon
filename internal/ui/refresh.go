package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// refreshWidth is the width of the progress box drawn over a refreshing tab.
const refreshWidth = 56

// refreshLog is how many finished items the progress box lists.
const refreshLog = 6

var (
	refreshBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).Padding(0, 1).Width(refreshWidth)
	doneMark    = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("✓")
	failedMark  = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("✗")
	spinnerTint = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
)

// refreshProgressMsg and refreshDoneMsg carry a running refresh's progress
// and outcome from its goroutine.
type (
	refreshProgressMsg Progress
	refreshDoneMsg     struct {
		summary string
		err     error
	}
)

// refreshing is a refresh in flight, drawn as a box of finished items, a
// spinner on the current one and an animated progress bar.
type refreshing struct {
	title    string
	updates  <-chan tea.Msg
	cancel   context.CancelFunc
	spinner  spinner.Model
	bar      progress.Model
	current  string
	done     int
	total    int
	finished []string
}

// startRefresh runs the dashboard's refresh off the UI loop.
func (a *App) startRefresh(d Dashboard) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	// Room for one progress message and the outcome, so the outcome is
	// never blocked once progress sends have given up.
	updates := make(chan tea.Msg, 2)
	report := a.report
	// Sends give up once cancelled, so quitting mid-refresh leaves no
	// goroutine blocked on a channel nobody reads.
	send := func(msg tea.Msg) {
		select {
		case updates <- msg:
		case <-ctx.Done():
		}
	}
	go func() {
		defer close(updates)
		summary, err := d.Refresh(ctx, report, func(p Progress) { send(refreshProgressMsg(p)) })
		// The outcome is sent even when cancelled, to close the box.
		updates <- refreshDoneMsg{summary: summary, err: err}
	}()

	a.refresh = &refreshing{
		title:   d.Title,
		updates: updates,
		cancel:  cancel,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(spinnerTint)),
		bar: progress.New(progress.WithDefaultBlend(), progress.WithoutPercentage(),
			progress.WithWidth(refreshWidth-4)),
	}
	return tea.Batch(a.waitRefresh(), a.refresh.spinner.Tick)
}

// cancelRefresh stops the running refresh, if any. Its outcome still arrives
// and closes the progress box.
func (a *App) cancelRefresh() {
	if a.refresh != nil {
		a.refresh.cancel()
	}
}

// waitRefresh waits for the running refresh's next message.
func (a *App) waitRefresh() tea.Cmd {
	updates := a.refresh.updates
	return func() tea.Msg { return <-updates }
}

// handleRefresh applies a message of the running refresh, reporting whether
// it was one. A finished refresh reloads the dashboards, keeping the view.
func (a *App) handleRefresh(msg tea.Msg) (tea.Cmd, bool) {
	r := a.refresh
	if r == nil {
		return nil, false
	}
	switch msg := msg.(type) {
	case refreshProgressMsg:
		return tea.Batch(r.advance(Progress(msg)), a.waitRefresh()), true
	case refreshDoneMsg:
		r.cancel()
		a.refresh = nil
		a.notice = refreshNotice(r.title, msg)
		return a.reload(true), true
	case spinner.TickMsg:
		var cmd tea.Cmd
		r.spinner, cmd = r.spinner.Update(msg)
		return cmd, true
	case progress.FrameMsg:
		var cmd tea.Cmd
		r.bar, cmd = r.bar.Update(msg)
		return cmd, true
	}
	return nil, false
}

// advance records progress and animates the bar towards it.
func (r *refreshing) advance(p Progress) tea.Cmd {
	r.current, r.done, r.total = p.Current, p.Done, p.Total
	if p.Finished != "" {
		mark := doneMark
		if p.Failed {
			mark = failedMark
		}
		line := fmt.Sprintf("%s %-12s %s", mark, p.Finished, footerStyle.Render(p.Detail))
		r.finished = append(r.finished, line)
	}
	if r.total == 0 {
		return nil
	}
	return r.bar.SetPercent(float64(r.done) / float64(r.total))
}

// view draws the progress box.
func (r *refreshing) view() string {
	lines := []string{brandStyle.Padding(0).Render("Refreshing " + r.title)}
	lines = append(lines, r.finished[max(len(r.finished)-refreshLog, 0):]...)
	if r.current != "" {
		lines = append(lines, r.spinner.View()+" "+r.current)
	}
	lines = append(lines, "", r.bar.View(),
		footerStyle.Render(fmt.Sprintf("%d/%d · esc to cancel", r.done, r.total)))
	return refreshBoxStyle.Render(strings.Join(lines, "\n"))
}

// refreshNotice is the footer line a finished refresh leaves.
func refreshNotice(title string, msg refreshDoneMsg) string {
	switch {
	case errors.Is(msg.err, context.Canceled):
		return strings.ToLower(title) + " refresh cancelled"
	case msg.err != nil:
		return strings.ToLower(title) + ": " + msg.err.Error()
	}
	return msg.summary
}
