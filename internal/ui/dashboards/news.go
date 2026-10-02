package dashboards

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// newsIDKey carries a row's article id, so enter can open it and a reload
// keeps the cursor on it. No column shows it.
const newsIDKey = "_news"

// copyLinkKey copies the article's link to the system clipboard.
const copyLinkKey = "y"

// copiedNote confirms a copied link in the footer.
const copiedNote = "link copied to system clipboard"

// newsTitleSymbols is how many symbols the table title names before "…".
const newsTitleSymbols = 4

var newsColumns = []widgets.Column{
	{Key: "published", Title: "Published", Width: 18, Left: true},
	{Key: "symbols", Title: "About", Flex: 2, Left: true},
	{Key: "source", Title: "Source", Flex: 2, Left: true},
	{Key: "title", Title: "Title", Flex: 9, Left: true},
}

var (
	articleTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230"))
	articleMetaStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	articleLinkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Underline(true)
)

// News lists the stored articles about the biggest holdings. Nothing is
// fetched until r is pressed on this tab; enter reads an article.
func News(report *portfolio.Report) ui.Component {
	if len(report.News) == 0 {
		return ui.Single(widgets.NewText("news", emptyNewsNote(report)))
	}
	return ui.Single(NewsTable(report))
}

// emptyNewsNote says why there is no news and how to get some.
func emptyNewsNote(report *portfolio.Report) string {
	if len(report.NewsSymbols) == 0 {
		return "No holdings to fetch news for."
	}
	return "No news stored yet. Press r to fetch the latest articles about " +
		strings.Join(report.NewsSymbols, ", ") + "."
}

// NewsTable lists articles newest first.
func NewsTable(report *portfolio.Report) *widgets.Table {
	rows := make([]widgets.Row, 0, len(report.News))
	for _, item := range report.News {
		rows = append(rows, widgets.Row{
			newsIDKey:                   item.ID,
			"published":                 dateTime(item.Published),
			widgets.SortBy("published"): float64(item.Published.Unix()),
			"symbols":                   strings.Join(item.Symbols, ", "),
			"source":                    item.Source,
			"title":                     item.Title,
		})
	}
	title := fmt.Sprintf("news — %d articles about %s · r fetch · enter read · y copy link · / filter",
		len(rows), symbolList(report.NewsSymbols))
	return widgets.NewTable(title, newsColumns, rows).
		Filterable().
		SortedBy("published", true).
		KeyedBy(newsIDKey).
		OnRowKey(copyLinkKey, func(row widgets.Row) tea.Cmd {
			item, ok := findNews(report, fmt.Sprint(row[newsIDKey]))
			if !ok {
				return nil
			}
			return copyLink(item.URL)
		}).
		OnSelect(func(row widgets.Row) tea.Cmd {
			id, ok := row[newsIDKey].(string)
			if !ok {
				return nil
			}
			item, ok := findNews(report, id)
			if !ok {
				return nil
			}
			return ui.Push(item.Source, func(report *portfolio.Report) (ui.Component, bool) {
				item, ok := findNews(report, id)
				if !ok {
					return nil, false
				}
				return NewsArticle(item), true
			})
		})
}

// symbolList names the first few symbols, eliding the rest.
func symbolList(symbols []string) string {
	if len(symbols) <= newsTitleSymbols {
		return strings.Join(symbols, ", ")
	}
	return strings.Join(symbols[:newsTitleSymbols], ", ") + ", …"
}

// findNews looks an article up by id.
func findNews(report *portfolio.Report, id string) (model.NewsItem, bool) {
	for _, item := range report.News {
		if item.ID == id {
			return item, true
		}
	}
	return model.NewsItem{}, false
}

// copyLink puts url on the system clipboard, through the terminal so it works
// over SSH too, and confirms it in the footer.
func copyLink(url string) tea.Cmd {
	return tea.Batch(tea.SetClipboard(url), ui.Notice(copiedNote))
}

// articlePager is an article's pager that also copies its link on y.
type articlePager struct {
	*widgets.Pager
	url string
}

// Update implements ui.Component.
func (a *articlePager) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == copyLinkKey {
		return copyLink(a.url)
	}
	return a.Pager.Update(msg)
}

// NewsArticle reads one article in a pager; y copies its link. The link is
// also a terminal hyperlink, opened with ⌘-click where the terminal supports
// it and shown as text where it does not.
func NewsArticle(item model.NewsItem) ui.Component {
	return &articlePager{Pager: newsArticlePager(item), url: item.URL}
}

func newsArticlePager(item model.NewsItem) *widgets.Pager {
	return widgets.NewPager("article", func(width int) string {
		wrap := lipgloss.NewStyle().Width(max(width, 1))
		meta := []string{item.Source, dateTime(item.Published) + " UTC"}
		if item.Premium {
			meta = append(meta, "premium")
		}
		parts := []string{
			wrap.Inherit(articleTitleStyle).Render(item.Title),
			wrap.Inherit(articleMetaStyle).Render(strings.Join(meta, " · ")),
			wrap.Inherit(articleMetaStyle).Render("about " + strings.Join(item.Symbols, ", ")),
			"",
			wrap.Render(item.Description),
			"",
			wrap.Inherit(articleLinkStyle).Hyperlink(item.URL).Render("open article ↗ " + item.URL),
		}
		if len(item.RelatedTickers) > 0 {
			parts = append(parts, "", wrap.Inherit(articleMetaStyle).
				Render("mentions "+strings.Join(item.RelatedTickers, " · ")))
		}
		return strings.Join(parts, "\n")
	}).WithHint("y copy link")
}
