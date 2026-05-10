package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// HomeAction is returned by HomeView.Render to tell the caller what the user selected.
type HomeAction int

const (
	HomeActionNone      HomeAction = iota // user quit without selecting
	HomeActionInstamart                   // user selected Instamart
)

// HomeView renders the main home/menu screen.
type HomeView struct {
	In io.Reader // if nil, static render (no keyboard input)
}

type homeItem struct {
	label     string
	available bool
}

var homeItems = []homeItem{
	{"Instamart", true},
	{"Food", false},
	{"Reorder usuals", false},
	{"Track orders", false},
	{"Addresses", false},
	{"Account", false},
}

type homeModel struct {
	ctx      context.Context
	viewport Viewport
	cursor   int
	items    []homeItem
	action   HomeAction
	notice   string // transient "coming soon" message
}

func (m homeModel) Init() tea.Cmd {
	return ctxQuitCmd(m.ctx)
}

func (m homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.action = HomeActionNone
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.notice = ""
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.notice = ""
			}
		case "enter", " ":
			item := m.items[m.cursor]
			if item.available {
				m.action = HomeActionInstamart
				return m, tea.Quit
			}
			m.notice = item.label + " — coming soon"
		}
	}
	return m, nil
}

func (m homeModel) View() string {
	logo := []string{
		"███████╗██╗    ██╗██╗ ██████╗  ██████╗ ██╗   ██╗",
		"██╔════╝██║    ██║██║██╔════╝ ██╔════╝ ╚██╗ ██╔╝",
		"███████╗██║ █╗ ██║██║██║  ███╗██║  ███╗ ╚████╔╝",
		"╚════██║██║███╗██║██║██║   ██║██║   ██║  ╚██╔╝",
		"███████║╚███╔███╔╝██║╚██████╔╝╚██████╔╝   ██║",
		"╚══════╝ ╚══╝╚══╝ ╚═╝ ╚═════╝  ╚═════╝    ╚═╝",
	}

	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(headerLine(" swiggy.ssh", connStyle.Render("● Connected SSH ")))
	sb.WriteString(divider())
	sb.WriteString(line(""))
	for _, l := range logo {
		sb.WriteString(centeredLine(brandStyle.Render(l)))
	}
	sb.WriteString(line(""))
	sb.WriteString(line(creamStyle.Render("                    Order groceries from your terminal")))
	sb.WriteString(line(accentStyle.Render("                    Instamart, straight from SSH")))
	sb.WriteString(line(""))
	sb.WriteString(divider())
	sb.WriteString(line(brandStyle.Render(" What would you like to do?")))
	sb.WriteString(line(""))
	for i, item := range m.items {
		label := fmt.Sprintf("%d. %s", i+1, item.label)
		if !item.available {
			label += "  " + mutedStyle.Render("(coming soon)")
		}
		if m.cursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			if item.available {
				sb.WriteString(line("   " + label))
			} else {
				sb.WriteString(line(mutedStyle.Render("   " + label)))
			}
		}
	}
	sb.WriteString(line(""))
	if m.notice != "" {
		sb.WriteString(line(" " + accentStyle.Render("⚡ "+m.notice)))
	} else {
		sb.WriteString(line(""))
	}
	sb.WriteString(divider())
	sb.WriteString(footerLine(
		KeyHint{Key: "j/k", Label: "move"},
		KeyHint{Key: "enter", Label: "select"},
		KeyHint{Key: "q", Label: "quit"},
	))
	sb.WriteString(bottom())
	return centerInViewport(sb.String(), m.viewport)
}

func (v HomeView) Render(ctx context.Context, w io.Writer) error {
	in := v.In
	if in == nil {
		in = strings.NewReader("")
	}
	m := homeModel{ctx: ctx, viewport: viewportFromContext(ctx), cursor: 0, items: homeItems}
	p := tea.NewProgram(m,
		tea.WithOutput(w),
		tea.WithInput(in),
		tea.WithoutSignals(),
	)
	_, err := p.Run()
	return err
}

// RenderWithAction runs HomeView and returns what the user selected.
func (v HomeView) RenderWithAction(ctx context.Context, w io.Writer) (HomeAction, error) {
	in := v.In
	if in == nil {
		in = strings.NewReader("")
	}
	m := homeModel{ctx: ctx, viewport: viewportFromContext(ctx), cursor: 0, items: homeItems}
	p := tea.NewProgram(m,
		tea.WithOutput(w),
		tea.WithInput(in),
		tea.WithoutSignals(),
	)
	finalModel, err := p.Run()
	if err != nil {
		return HomeActionNone, err
	}
	if hm, ok := finalModel.(homeModel); ok {
		return hm.action, nil
	}
	return HomeActionNone, nil
}
