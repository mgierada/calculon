package dashboards

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// ratings name the analyst ratings from best to worst, with their colours;
// a month's score is the mean of their positions, 1 to 5.
var ratings = []struct {
	name  string
	color string
	count func(model.Recommendation) int
}{
	{"Strong buy", "34", func(r model.Recommendation) int { return r.StrongBuy }},
	{"Buy", "42", func(r model.Recommendation) int { return r.Buy }},
	{"Hold", "220", func(r model.Recommendation) int { return r.Hold }},
	{"Sell", "208", func(r model.Recommendation) int { return r.Sell }},
	{"Strong sell", "196", func(r model.Recommendation) int { return r.StrongSell }},
}

var recommendationColumns = []widgets.Column{
	{Key: "month", Title: "Month", Flex: 2, Left: true},
	{Key: "consensus", Title: "Consensus", Flex: 2, Left: true},
	{Key: "score", Title: "Score", Flex: 1},
	{Key: "change", Title: "Change", Flex: 1},
	{Key: "analysts", Title: "Analysts", Flex: 1},
	{Key: "buy", Title: "Buy %", Flex: 1},
	{Key: "hold", Title: "Hold %", Flex: 1},
	{Key: "sell", Title: "Sell %", Flex: 1},
}

// recommendationsSelectMsg tells the recommendations view which stock was
// picked.
type recommendationsSelectMsg string

// recommendationsState is what a recommendations view keeps across reloads.
type recommendationsState struct {
	symbol string
	offset int
}

// RecommendationsView shows one held stock's analyst recommendations over
// time: the latest month's split, the counts of every month and how the
// consensus moved. p picks the stock from the holdings; a stock with stored
// data shows it at once, one without is fetched. Nothing is fetched until a
// stock is picked or r is pressed.
type RecommendationsView struct {
	report  *portfolio.Report
	symbol  string
	body    ui.Component
	matrix  *widgets.Matrix
	history *widgets.Table
	width   int
	height  int
	focused bool
}

// Recommendations builds the view on the most recently fetched stock.
func Recommendations(report *portfolio.Report) ui.Component {
	view := &RecommendationsView{report: report}
	view.show(latestRecommendations(report))
	return view
}

// latestRecommendations is the held symbol whose recommendations were fetched
// last.
func latestRecommendations(report *portfolio.Report) string {
	latest, symbol := time.Time{}, ""
	for s, r := range report.Recommendations {
		if r.FetchedAt.After(latest) || (r.FetchedAt.Equal(latest) && s < symbol) {
			latest, symbol = r.FetchedAt, s
		}
	}
	return symbol
}

// show switches to symbol, building its panels from the stored
// recommendations.
func (v *RecommendationsView) show(symbol string) {
	v.symbol, v.matrix, v.history = symbol, nil, nil
	recs, ok := v.report.Recommendations[symbol]
	switch {
	case symbol == "":
		v.body = ui.Single(widgets.NewText("recommendations",
			"Press p to pick a stock you hold and see its analyst recommendations."))
	case !ok:
		v.body = ui.Single(widgets.NewText("recommendations "+symbol,
			"No recommendations stored for "+symbol+" yet. Press r to fetch them, or p to pick another stock."))
	default:
		latest := recs.Months[len(recs.Months)-1]
		v.matrix, v.history = ratingMatrix(recs), recommendationHistory(recs)
		v.body = &ui.Grid{Rows: []ui.Row{
			{Height: statRowHeight + 1, Cells: []ui.Cell{
				{Component: recommendationsHeader(v.report, recs), Weight: 2},
				{Component: ratingSplit(latest), Weight: 5},
			}},
			{Height: v.matrix.Height(), Cells: ui.Cells(v.matrix)},
			{Weight: 1, Cells: ui.Cells(v.history)},
		}}
	}
	v.body.SetSize(v.width, v.height)
	v.body.SetFocused(v.focused)
}

// Init implements ui.Component.
func (v *RecommendationsView) Init() tea.Cmd { return nil }

// Update implements ui.Component: p picks a stock, ← → scroll the months
// and other keys scroll the history table.
func (v *RecommendationsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case recommendationsSelectMsg:
		v.show(string(msg))
		if _, stored := v.report.Recommendations[v.symbol]; !stored {
			return ui.RequestRefresh()
		}
		return nil
	case tea.KeyPressMsg:
		key := msg.String()
		if key == pickStockKey {
			return ui.Push("pick a stock", holdingPicker(recommendationsFetched,
				func(symbol string) tea.Msg { return recommendationsSelectMsg(symbol) }))
		}
		if v.matrix != nil && v.matrix.Scroll(key) {
			return nil
		}
		if v.history != nil {
			return v.history.Update(msg)
		}
	}
	return nil
}

// RefreshTarget implements ui.RefreshTargeter: r refetches the shown stock.
func (v *RecommendationsView) RefreshTarget() string { return v.symbol }

// State implements ui.Stateful, keeping the stock and scroll across reloads.
func (v *RecommendationsView) State() any {
	state := recommendationsState{symbol: v.symbol}
	if v.matrix != nil {
		state.offset = v.matrix.Offset()
	}
	return state
}

// Restore implements ui.Stateful.
func (v *RecommendationsView) Restore(state any) {
	s, ok := state.(recommendationsState)
	if !ok {
		return
	}
	v.show(s.symbol)
	if v.matrix != nil {
		v.matrix.SetOffset(s.offset)
	}
}

// SetSize implements ui.Component.
func (v *RecommendationsView) SetSize(width, height int) {
	v.width, v.height = width, height
	v.body.SetSize(width, height)
}

// SetFocused implements ui.Component.
func (v *RecommendationsView) SetFocused(focused bool) {
	v.focused = focused
	v.body.SetFocused(focused)
}

// View implements ui.Component.
func (v *RecommendationsView) View() string { return v.body.View() }

// recommendationsFetched is when a symbol's stored recommendations were
// fetched.
func recommendationsFetched(report *portfolio.Report, symbol string) (time.Time, bool) {
	r, ok := report.Recommendations[symbol]
	return r.FetchedAt, ok
}

// recommendationsHeader names the stock and its latest consensus.
func recommendationsHeader(report *portfolio.Report, recs model.Recommendations) *widgets.Stat {
	latest := recs.Months[len(recs.Months)-1]
	value := consensus(latest)
	if analysts := latest.Analysts(); analysts > 0 {
		value += fmt.Sprintf(" · %d analysts", analysts)
	}
	return widgets.NewStat(recs.Symbol+" · "+holdingName(report, recs.Symbol), value).
		WithNote(fmt.Sprintf("%d months · fetched %s · p change", len(recs.Months),
			dateTime(recs.FetchedAt)), widgets.Muted)
}

// ratingSplit shows how the analysts split in a month.
func ratingSplit(r model.Recommendation) *widgets.StackBar {
	segments := make([]widgets.Bar, len(ratings))
	for i, rating := range ratings {
		segments[i] = widgets.Bar{Label: rating.name, Value: float64(rating.count(r)),
			Color: lipgloss.Color(rating.color)}
	}
	return widgets.NewStackBar(monthLabel(r.Month), segments)
}

// ratingMatrix lays every rating's count, the analysts and the score out by
// month.
func ratingMatrix(recs model.Recommendations) *widgets.Matrix {
	labels := make([]string, len(recs.Months))
	for i, r := range recs.Months {
		labels[i] = monthLabel(r.Month)
	}
	count := func(v float64) string { return strconv.Itoa(int(v)) }
	rows := make([]widgets.MatrixRow, 0, len(ratings)+2)
	for _, rating := range ratings {
		rows = append(rows, widgets.MatrixRow{Label: rating.name, Format: count,
			Values: monthly(recs, func(r model.Recommendation) *float64 {
				return counted(rating.count(r))
			})})
	}
	rows = append(rows,
		widgets.MatrixRow{Label: "Analysts", Format: count, Bold: true,
			Values: monthly(recs, func(r model.Recommendation) *float64 {
				return counted(r.Analysts())
			})},
		widgets.MatrixRow{Label: "Score", Format: score, Bold: true,
			Values: monthly(recs, func(r model.Recommendation) *float64 {
				s, ok := r.Score()
				if !ok {
					return nil
				}
				return &s
			})},
	)
	return widgets.NewMatrix("analysts by month · ← → scroll", "score 1 strong buy – 5 strong sell",
		labels, rows...)
}

// counted is a count as a matrix value.
func counted(n int) *float64 {
	v := float64(n)
	return &v
}

// monthly is one figure of every month, oldest first.
func monthly(recs model.Recommendations, figure func(model.Recommendation) *float64) []*float64 {
	values := make([]*float64, len(recs.Months))
	for i, r := range recs.Months {
		values[i] = figure(r)
	}
	return values
}

// recommendationHistory lists every month's consensus, newest first, with
// how the score moved since the month before.
func recommendationHistory(recs model.Recommendations) *widgets.Table {
	rows := make([]widgets.Row, 0, len(recs.Months))
	for i, r := range slices.Backward(recs.Months) {
		s, rated := r.Score()
		row := widgets.Row{
			"month":     monthLabel(r.Month),
			"consensus": widgets.Toned(consensus(r), consensusTone(s, rated)),
			"score":     widgets.Toned(unknown, widgets.Muted),
			"change":    widgets.Toned(unknown, widgets.Muted),
			"analysts":  strconv.Itoa(r.Analysts()),
			"buy":       share(r.StrongBuy+r.Buy, r.Analysts()),
			"hold":      share(r.Hold, r.Analysts()),
			"sell":      share(r.Sell+r.StrongSell, r.Analysts()),
		}
		if rated {
			row["score"] = score(s)
		}
		if i > 0 {
			if before, ok := recs.Months[i-1].Score(); ok && rated {
				// A lower score is a better rating, so a fall reads as good news.
				change := s - before
				row["change"] = widgets.Toned(fmt.Sprintf("%+.2f", change),
					widgets.ToneOf(-roundedScore(change)))
			}
		}
		rows = append(rows, row)
	}
	return widgets.NewTable(fmt.Sprintf("consensus by month · %d months", len(rows)),
		recommendationColumns, rows)
}

// consensus names the rating nearest the month's score.
func consensus(r model.Recommendation) string {
	s, ok := r.Score()
	if !ok {
		return "No ratings"
	}
	return ratings[min(max(int(math.Round(s))-1, 0), len(ratings)-1)].name
}

// consensusTone colours a buy consensus green and a sell one red.
func consensusTone(s float64, rated bool) widgets.Tone {
	if !rated {
		return widgets.Muted
	}
	return widgets.ToneOf(roundedScore(3 - s))
}

// roundedScore drops score noise below what is shown, so an unchanged
// consensus reads neutral.
func roundedScore(v float64) float64 { return math.Round(v*100) / 100 }

// score prints a mean rating, e.g. 1.75.
func score(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }

// share is part of whole as a percentage, unknown when whole is zero.
func share(part, whole int) any {
	if whole == 0 {
		return widgets.Toned(unknown, widgets.Muted)
	}
	return percent(float64(part) / float64(whole) * 100)
}

// monthLabel names a month, e.g. Sep '26.
func monthLabel(month time.Time) string { return month.Format("Jan '06") }
