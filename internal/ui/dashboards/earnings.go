package dashboards

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// pickStockKey opens the stock picker on the earnings tab.
const pickStockKey = "p"

// benchmarkNames are readable names of the indexes growth is compared with.
var benchmarkNames = map[string]string{"SP5": "S&P 500"}

var epsColumns = []widgets.Column{
	{Key: "period", Title: "Period", Flex: 2, Left: true},
	{Key: "actual", Title: "EPS actual", Flex: 2},
	{Key: "estimate", Title: "Estimate", Flex: 2},
	{Key: "surprise", Title: "Surprise", Flex: 2},
	{Key: "surprise_pct", Title: "Surprise %", Flex: 2},
}

// earningsSelectMsg tells the earnings view which stock was picked.
type earningsSelectMsg string

// earningsState is what an earnings view keeps across reloads.
type earningsState struct {
	symbol string
	offset int
}

// EarningsView shows one held stock's earnings: the analysts' price targets,
// revenue and earnings by period, and EPS against estimates. p picks the
// stock from the holdings; a stock with stored data shows it at once, one
// without is fetched. Nothing is fetched until a stock is picked or r is
// pressed.
type EarningsView struct {
	report  *portfolio.Report
	symbol  string
	body    ui.Component
	matrix  *widgets.Matrix
	eps     *widgets.Table
	width   int
	height  int
	focused bool
}

// Earnings builds the view on the most recently fetched stock.
func Earnings(report *portfolio.Report) ui.Component {
	view := &EarningsView{report: report}
	view.show(latestEarnings(report))
	return view
}

// latestEarnings is the held symbol whose earnings were fetched last.
func latestEarnings(report *portfolio.Report) string {
	latest, symbol := time.Time{}, ""
	for s, e := range report.Earnings {
		if e.FetchedAt.After(latest) || (e.FetchedAt.Equal(latest) && s < symbol) {
			latest, symbol = e.FetchedAt, s
		}
	}
	return symbol
}

// show switches to symbol, building its panels from the stored earnings.
func (v *EarningsView) show(symbol string) {
	v.symbol, v.matrix, v.eps = symbol, nil, nil
	e, ok := v.report.Earnings[symbol]
	switch {
	case symbol == "":
		v.body = ui.Single(widgets.NewText("earnings",
			"Press p to pick a stock you hold and see its earnings and price targets."))
	case !ok:
		v.body = ui.Single(widgets.NewText("earnings "+symbol,
			"No earnings stored for "+symbol+" yet. Press r to fetch them, or p to pick another stock."))
	default:
		v.matrix, v.eps = financials(e), epsTable(e)
		v.body = &ui.Grid{Rows: []ui.Row{
			{Height: statRowHeight + 1, Cells: []ui.Cell{
				{Component: earningsHeader(v.report, e), Weight: 2},
				{Component: targetSlider(e), Weight: 5},
			}},
			{Height: v.matrix.Height(), Cells: ui.Cells(v.matrix)},
			{Weight: 1, Cells: ui.Cells(v.eps)},
		}}
	}
	v.body.SetSize(v.width, v.height)
	v.body.SetFocused(v.focused)
}

// Init implements ui.Component.
func (v *EarningsView) Init() tea.Cmd { return nil }

// Update implements ui.Component: p picks a stock, ← → scroll the periods
// and other keys scroll the EPS table.
func (v *EarningsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case earningsSelectMsg:
		v.show(string(msg))
		if _, stored := v.report.Earnings[v.symbol]; !stored {
			return ui.RequestRefresh()
		}
		return nil
	case tea.KeyPressMsg:
		key := msg.String()
		if key == pickStockKey {
			return ui.Push("pick a stock", stockPicker)
		}
		if v.matrix != nil && v.matrix.Scroll(key) {
			return nil
		}
		if v.eps != nil {
			return v.eps.Update(msg)
		}
	}
	return nil
}

// RefreshTarget implements ui.RefreshTargeter: r refetches the shown stock.
func (v *EarningsView) RefreshTarget() string { return v.symbol }

// State implements ui.Stateful, keeping the stock and scroll across reloads.
func (v *EarningsView) State() any {
	state := earningsState{symbol: v.symbol}
	if v.matrix != nil {
		state.offset = v.matrix.Offset()
	}
	return state
}

// Restore implements ui.Stateful.
func (v *EarningsView) Restore(state any) {
	s, ok := state.(earningsState)
	if !ok {
		return
	}
	v.show(s.symbol)
	if v.matrix != nil {
		v.matrix.SetOffset(s.offset)
	}
}

// SetSize implements ui.Component.
func (v *EarningsView) SetSize(width, height int) {
	v.width, v.height = width, height
	v.body.SetSize(width, height)
}

// SetFocused implements ui.Component.
func (v *EarningsView) SetFocused(focused bool) {
	v.focused = focused
	v.body.SetFocused(focused)
}

// View implements ui.Component.
func (v *EarningsView) View() string { return v.body.View() }

// stockPicker lists the held stocks, biggest first, noting which have stored
// earnings; choosing one closes the picker and shows it.
func stockPicker(report *portfolio.Report) (ui.Component, bool) {
	value, names := map[string]float64{}, map[string]string{}
	for _, h := range report.Holdings {
		value[h.Symbol] += h.ValueBase
		names[h.Symbol] = h.Name
	}
	symbols := portfolio.TopSymbols(report.Holdings, len(report.Holdings))
	items := make([]widgets.ListItem, 0, len(symbols))
	for _, symbol := range symbols {
		detail := fmt.Sprintf("%s · %s", names[symbol], money(value[symbol], report.Base))
		if e, ok := report.Earnings[symbol]; ok {
			detail += " · stored " + date(e.FetchedAt)
		}
		items = append(items, widgets.ListItem{Name: symbol, Detail: detail, Value: symbol})
	}
	return widgets.NewListPicker("Pick a stock", items, func(item widgets.ListItem) tea.Cmd {
		symbol := fmt.Sprint(item.Value)
		return tea.Sequence(ui.Pop(), func() tea.Msg { return earningsSelectMsg(symbol) })
	}), true
}

// earningsHeader names the stock and says how current its data is.
func earningsHeader(report *portfolio.Report, e model.Earnings) *widgets.Stat {
	name := e.Symbol
	for _, h := range report.Holdings {
		if h.Symbol == e.Symbol {
			name = h.Name
			break
		}
	}
	return widgets.NewStat(e.Symbol+" · "+name, fmt.Sprintf("%d data points", e.TotalCount)).
		WithNote("fetched "+dateTime(e.FetchedAt)+" · p change", widgets.Muted)
}

// targetSlider places the analysts' low, average and high targets and the
// target price on one track.
func targetSlider(e model.Earnings) *widgets.RangeSlider {
	t := e.Target
	slider := widgets.NewRangeSlider("analyst price target",
		widgets.LowMark(t.Low), widgets.HighMark(t.High), widgets.AverageMark(t.Average),
		widgets.TargetMark(t.Price)).WithFormat(price)
	if t.Average != nil && t.Price != nil && *t.Price != 0 {
		slider.WithNote(fmt.Sprintf("average target is %s against the target price",
			signedPercent((*t.Average-*t.Price) / *t.Price * 100)))
	}
	return slider
}

// period identifies a reporting period across item kinds.
type period struct {
	start  time.Time
	length string
}

// financials lays revenue, earnings and growth out by period.
func financials(e model.Earnings) *widgets.Matrix {
	periods := earningsPeriods(e)
	labels := make([]string, len(periods))
	for i, p := range periods {
		labels[i] = periodLabel(p)
	}
	note := "data in mln"
	if e.Currency != "" {
		note += " " + e.Currency
	}
	return widgets.NewMatrix("revenue and earnings · ← → scroll", note, labels, financialRows(e, periods)...)
}

// financialRows are the figures of each period, oldest first.
func financialRows(e model.Earnings, periods []period) []widgets.MatrixRow {
	index := map[period]int{}
	for i, p := range periods {
		index[p] = i
	}
	values := func() []*float64 { return make([]*float64, len(periods)) }
	revenue, earned, margin, growth, benchmark := values(), values(), values(), values(), values()
	for _, r := range e.Revenue {
		i, ok := index[period{r.Start, r.Length}]
		if !ok {
			continue
		}
		revenue[i], earned[i] = scaled(r.Revenue, 1e-6), scaled(r.Earnings, 1e-6)
		if r.Revenue != nil && r.Earnings != nil && *r.Revenue != 0 {
			pct := *r.Earnings / *r.Revenue * 100
			margin[i] = &pct
		}
	}
	benchmarkName := "benchmark"
	for _, g := range e.Growth {
		if i, ok := index[period{g.Start, g.Length}]; ok {
			growth[i], benchmark[i] = scaled(g.Growth, 100), scaled(g.Benchmark, 100)
		}
		if name, ok := benchmarkNames[g.BenchmarkSymbol]; ok {
			benchmarkName = name
		} else if g.BenchmarkSymbol != "" {
			benchmarkName = g.BenchmarkSymbol
		}
	}
	millions := func(v float64) string { return groupThousands(strconv.FormatFloat(v, 'f', 1, 64)) }
	return []widgets.MatrixRow{
		{Label: "Revenue", Values: revenue, Format: millions, Bold: true},
		{Label: "Net earnings", Values: earned, Format: millions, Bold: true, Toned: true},
		{Label: "Net margin", Values: margin, Format: percent, Toned: true},
		{Label: "Revenue growth", Values: growth, Format: signedPercent, Toned: true},
		{Label: benchmarkName + " growth", Values: benchmark, Format: signedPercent, Toned: true},
	}
}

// earningsPeriods are every period any figure covers, oldest first.
func earningsPeriods(e model.Earnings) []period {
	seen := map[period]bool{}
	var periods []period
	add := func(p model.EarningsPeriod) {
		key := period{p.Start, p.Length}
		if !seen[key] {
			seen[key] = true
			periods = append(periods, key)
		}
	}
	for _, r := range e.Revenue {
		add(r.EarningsPeriod)
	}
	for _, g := range e.Growth {
		add(g.EarningsPeriod)
	}
	slices.SortFunc(periods, func(a, b period) int { return a.start.Compare(b.start) })
	return periods
}

// periodLabel names a period the way statements do, e.g. Q2 '26 or 2026.
func periodLabel(p period) string {
	switch p.length {
	case "quarter":
		return fmt.Sprintf("Q%d '%02d", (int(p.start.Month())-1)/3+1, p.start.Year()%100)
	case "year":
		return strconv.Itoa(p.start.Year())
	}
	return p.start.Format("2006-01")
}

// scaled multiplies a known value, keeping unknown ones nil.
func scaled(v *float64, factor float64) *float64 {
	if v == nil {
		return nil
	}
	s := *v * factor
	return &s
}

// epsTable lists earnings per share against estimates, newest first.
func epsTable(e model.Earnings) *widgets.Table {
	rows := make([]widgets.Row, 0, len(e.EPS))
	for _, eps := range slices.Backward(e.EPS) {
		rows = append(rows, widgets.Row{
			"period":       periodLabel(period{eps.Start, eps.Length}),
			"actual":       optional(eps.Actual, price),
			"estimate":     optional(eps.Estimate, price),
			"surprise":     tonedOptional(eps.Surprise, price),
			"surprise_pct": tonedOptional(eps.SurprisePct, signedPercent),
		})
	}
	return widgets.NewTable(fmt.Sprintf("earnings per share · %d periods", len(rows)), epsColumns, rows)
}

// optional formats a value that may be unknown.
func optional(v *float64, format func(float64) string) any {
	if v == nil || math.IsNaN(*v) {
		return widgets.Toned(unknown, widgets.Muted)
	}
	return format(*v)
}

// tonedOptional formats a value that may be unknown, coloured by its sign.
func tonedOptional(v *float64, format func(float64) string) any {
	if v == nil {
		return widgets.Toned(unknown, widgets.Muted)
	}
	return widgets.Toned(format(*v), widgets.ToneOf(*v))
}
