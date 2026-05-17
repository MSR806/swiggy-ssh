package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

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
	{"swiggy.ai", false},
	{"Track orders", false},
	{"Addresses", false},
}

var homeLogoGradient = []string{"#E97112", "#FC8019", "#FF8B2E", "#FF9843"}

type homeSplashTickMsg struct{}

const homeSplashTickInterval = 500 * time.Millisecond

type homeModel struct {
	ctx       context.Context
	viewport  Viewport
	cursor    int
	items     []homeItem
	action    HomeAction
	notice    string // transient "coming soon" message
	menu      bool
	animate   bool
	shineStep int
}

func (m homeModel) Init() tea.Cmd {
	if m.animate && !m.menu {
		return tea.Batch(ctxQuitCmd(m.ctx), homeSplashTickCmd())
	}
	return ctxQuitCmd(m.ctx)
}

func homeSplashTickCmd() tea.Cmd {
	return tea.Tick(homeSplashTickInterval, func(time.Time) tea.Msg {
		return homeSplashTickMsg{}
	})
}

func (m homeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case homeSplashTickMsg:
		if !m.menu && m.animate {
			m.shineStep = (m.shineStep + 1) % homeSplashShineFrames
			return m, homeSplashTickCmd()
		}
		return m, nil
	case tea.KeyMsg:
		if !m.menu {
			switch msg.String() {
			case "q", "ctrl+c":
				m.action = HomeActionNone
				return m, tea.Quit
			case "enter", " ":
				m.menu = true
				return m, nil
			}
			return m, nil
		}
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

const homeSplashShineFrames = 11

func homeSplashRenderLine(content string, row, total, shineStep int) string {
	if row == (shineStep+2)%homeSplashShineFrames {
		return shineStyle.Render(content)
	}
	return gradientRender(content, homeLogoGradient, row, total)
}

func homeSplashContinueHint(shineStep int) KeyHint {
	markers := []struct {
		left  string
		right string
	}{
		{"▸", "◂"},
		{"›", "‹"},
		{"»", "«"},
		{"›", "‹"},
	}
	marker := markers[shineStep%len(markers)]
	return KeyHint{Key: marker.left + " enter", Label: "continue " + marker.right, Highlight: true}
}

func (m homeModel) View() string {
	logo := []string{
		"    ⢀⣠⣴⣶⣶⣶⣶⣦⣄⡀    ",
		"  ⢀⣴⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣦⡀ ",
		"  ⣾⣿⣿⣿⣿⣿⣿⡏⢹⣿⣿⣿⣿⣷ ",
		" ⢰⣿⣿⣿⣿⣿⣿⣿⡇⠸⠿⠿⠿⠿⠟⠃",
		" ⠈⣿⣿⣿⣿⣿⣿⣿⣷⣶⣶⣶⣶⣶⣤ ",
		"  ⢹⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡟ ",
		"   ⠻⢿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠁ ",
		"    ⢶⣶⣶⣶⣤⢸⣿⣿⣿⡿⠁  ",
		"     ⠹⣿⣿⣿⣼⣿⣿⡟⠁   ",
		"      ⠈⢿⣿⣿⡿⠋     ",
		"        ⠹⠏        ",
	}
	wordmark := []string{
		"███████╗██╗    ██╗██╗ ██████╗  ██████╗ ██╗   ██╗",
		"██╔════╝██║    ██║██║██╔════╝ ██╔════╝ ╚██╗ ██╔╝",
		"███████╗██║ █╗ ██║██║██║  ███╗██║  ███╗ ╚████╔╝ ",
		"╚════██║██║███╗██║██║██║   ██║██║   ██║  ╚██╔╝  ",
		"███████║╚███╔███╔╝██║╚██████╔╝╚██████╔╝   ██║   ",
		"╚══════╝ ╚══╝╚══╝ ╚═╝ ╚═════╝  ╚═════╝    ╚═╝   ",
	}

	// Keep the root menu inside the same 80x24 frame as Instamart.
	const logoLines = 11
	const wordmarkLines = 6
	const wordmarkWidth = 48
	const topPad = (logoLines - wordmarkLines) / 2
	const splashTopPad = (fixedFrameBodyRows - logoLines + 1) / 2

	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(headerLine(" swiggy.dev", connStyle.Render("● Connected SSH ")))
	sb.WriteString(divider())

	var body strings.Builder
	if !m.menu {
		for range splashTopPad {
			body.WriteString(line(""))
		}
		for i, logoLine := range logo {
			wmIdx := i - topPad
			right := strings.Repeat(" ", wordmarkWidth)
			if wmIdx >= 0 && wmIdx < wordmarkLines {
				right = gradientRender(wordmark[wmIdx], homeLogoGradient, wmIdx, wordmarkLines)
			}
			body.WriteString(centeredLine(homeSplashRenderLine(logoLine, i, logoLines, m.shineStep) + "  " + right))
		}
		sb.WriteString(fixedBody(body.String(), fixedFrameBodyRows))
		sb.WriteString(divider())
		sb.WriteString(footerLine(
			homeSplashContinueHint(m.shineStep),
			KeyHint{Key: "q", Label: "quit"},
		))
		sb.WriteString(bottom())
		return centerInViewport(sb.String(), m.viewport)
	}

	body.WriteString(line(""))
	body.WriteString(line(""))
	body.WriteString(line(""))
	body.WriteString(line(brandStyle.Render(" What would you like to do?")))
	for i, item := range m.items {
		label := fmt.Sprintf("%d. %s", i+1, item.label)
		if !item.available {
			label += "  " + mutedStyle.Render("(coming soon)")
		}
		if m.cursor == i {
			body.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			if item.available {
				body.WriteString(line("   " + label))
			} else {
				body.WriteString(line(mutedStyle.Render("   " + label)))
			}
		}
	}
	if m.notice != "" {
		body.WriteString(line(" " + accentStyle.Render("⚡ "+m.notice)))
	} else {
		body.WriteString(line(""))
	}
	sb.WriteString(fixedBody(body.String(), fixedFrameBodyRows))
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
	m := homeModel{ctx: ctx, viewport: viewportFromContext(ctx), cursor: 0, items: homeItems, animate: v.In != nil}
	_, err := runInteractive(m, w, v.In)
	return err
}

// RenderWithAction runs HomeView and returns what the user selected.
func (v HomeView) RenderWithAction(ctx context.Context, w io.Writer) (HomeAction, error) {
	m := homeModel{ctx: ctx, viewport: viewportFromContext(ctx), cursor: 0, items: homeItems, animate: v.In != nil}
	finalModel, err := runInteractive(m, w, v.In)
	if err != nil {
		return HomeActionNone, err
	}
	if hm, ok := finalModel.(homeModel); ok {
		return hm.action, nil
	}
	return HomeActionNone, nil
}
