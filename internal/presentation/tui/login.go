package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"swiggy-ssh/internal/application/auth"
)

// LoginWaitingView renders the "open browser and enter code" prompt.
type LoginWaitingView struct {
	LoginURL string
	RawCode  string
}

func (v LoginWaitingView) Render(ctx context.Context, w io.Writer) error {
	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(line(" " + brandStyle.Render("swiggy.ssh") + creamStyle.Render(" > Login")))
	sb.WriteString(divider())
	sb.WriteString(line(""))
	sb.WriteString(line(" " + creamStyle.Render("You need to connect your Swiggy account before placing orders.")))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + brandStyle.Render("Open this URL in your browser:")))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + accentStyle.Render(v.LoginURL)))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + brandStyle.Render("Enter code:")))
	sb.WriteString(line(""))
	sb.WriteString(codeBox(v.RawCode))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + creamStyle.Render("Waiting for login...")))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + brandStyle.Render("Status:") + " " + mutedStyle.Render("not connected")))
	sb.WriteString(line(""))
	sb.WriteString(divider())
	sb.WriteString(footerLine(
		KeyHint{Key: "r", Label: "refresh"},
		KeyHint{Key: "c", Label: "copy URL"},
		KeyHint{Key: "b", Label: "back"},
		KeyHint{Key: "q", Label: "quit"},
	))
	sb.WriteString(bottom())
	return runStatic(w, centerInViewport(sb.String(), viewportFromContext(ctx)))
}

// LoginSuccessView renders the post-login confirmation.
type LoginSuccessView struct {
	IsFirstAuth bool
	WasReauth   bool
	Account     auth.OAuthAccount // kept for safety — never rendered
	DisplayName string            // shown as "Signed in as" name
	Email       string            // shown as email
	In          io.Reader         // if nil, static render
}

var loginSuccessChoices = []string{
	"Instamart",
	"Home",
	"Account settings",
}

type loginSuccessModel struct {
	ctx      context.Context
	viewport Viewport
	cursor   int
	choices  []string
	name     string
	email    string
}

func (m loginSuccessModel) Init() tea.Cmd {
	return ctxQuitCmd(m.ctx)
}

func (m loginSuccessModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
			}
		case "enter", "b":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m loginSuccessModel) View() string {
	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(line(" " + brandStyle.Render("swiggy.ssh") + creamStyle.Render(" > Login")))
	sb.WriteString(divider())
	sb.WriteString(line(""))
	sb.WriteString(line(" " + successStyle.Render("✓ Swiggy account connected")))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + brandStyle.Render("Signed in as:") + " " + boldStyle.Render(m.name) + mutedStyle.Render("  <"+m.email+">")))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + creamStyle.Render("You can now search Instamart, manage your cart, and place COD orders.")))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + brandStyle.Render("Continue to:")))
	sb.WriteString(line(""))
	for i, choice := range m.choices {
		label := fmt.Sprintf("%d. %s", i+1, choice)
		if m.cursor == i {
			sb.WriteString(line(cursorStyle.Render("> ") + boldStyle.Render(label)))
		} else {
			sb.WriteString(line("   " + label))
		}
	}
	sb.WriteString(line(""))
	sb.WriteString(divider())
	sb.WriteString(footerLine(
		KeyHint{Key: "j/k", Label: "move"},
		KeyHint{Key: "enter", Label: "select"},
		KeyHint{Key: "b", Label: "back"},
		KeyHint{Key: "q", Label: "quit"},
	))
	sb.WriteString(bottom())
	return centerInViewport(sb.String(), m.viewport)
}

func (v LoginSuccessView) Render(ctx context.Context, w io.Writer) error {
	name := v.DisplayName
	if name == "" {
		name = "Unknown"
	}
	email := v.Email
	if email == "" {
		email = "(no email)"
	}
	in := v.In
	if in == nil {
		in = strings.NewReader("")
	}
	m := loginSuccessModel{
		ctx:      ctx,
		viewport: viewportFromContext(ctx),
		cursor:   0,
		choices:  loginSuccessChoices,
		name:     name,
		email:    email,
	}
	p := tea.NewProgram(m,
		tea.WithOutput(w),
		tea.WithInput(in),
		tea.WithoutSignals(),
	)
	_, err := p.Run()
	return err
}

// ReconnectView renders the re-auth prompt shown before a new login code.
// Inline (no full-screen box): shown mid-session when re-auth is needed.
type ReconnectView struct {
	RawCode string
}

func (v ReconnectView) Render(ctx context.Context, w io.Writer) error {
	content := "\r\n" +
		"  " + brandStyle.Render("Your session needs re-authentication.") + "\r\n" +
		"  " + creamStyle.Render("Enter this code in the browser login page:") + "\r\n" +
		"\r\n" +
		"     " + accentStyle.Render(boldStyle.Render(v.RawCode)) + "\r\n" +
		"\r\n" +
		"  " + creamStyle.Render("Waiting...") + "\r\n"
	_, err := fmt.Fprint(w, centerInViewport(content, viewportFromContext(ctx)))
	return err
}
