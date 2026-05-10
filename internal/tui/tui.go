package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"swiggy-ssh/internal/auth"
)

// View is the terminal screen boundary for the SSH delivery adapter.
type View interface {
	Render(ctx context.Context, w io.Writer) error
}

// --- Lipgloss styles ---

var (
	accentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FC8019"))         // Swiggy orange
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA44")).Bold(true) // green bold
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))              // gray
	boldStyle    = lipgloss.NewStyle().Bold(true)
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FC8019")).Bold(true) // orange + bold
	codeStyle    = lipgloss.NewStyle().Bold(true)
	connStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00AA44"))           // green
)

// --- Box-drawing helpers ---
// All screens are 80 characters wide (including │ borders):
//
//	┌ + 78×─ + ┐
//	│ + 78 chars of content + │
//	└ + 78×─ + ┘

const innerWidth = 78

func top() string {
	return "┌" + strings.Repeat("─", innerWidth) + "┐\r\n"
}

func bottom() string {
	return "└" + strings.Repeat("─", innerWidth) + "┘\r\n"
}

func divider() string {
	return "├" + strings.Repeat("─", innerWidth) + "┤\r\n"
}

// line returns │ + (space + content right-padded to total 78 inner display columns) + │\r\n.
// Width is measured with lipgloss.Width so ANSI escape sequences from lipgloss styling
// are excluded from the measurement and the box stays exactly 80 chars wide.
func line(content string) string {
	inner := " " + content
	w := lipgloss.Width(inner)
	if w > innerWidth {
		// Hard-truncate visible characters; keep trailing reset if styled.
		// For oversized raw strings we slice runes; styled content shouldn't overflow.
		runes := []rune(inner)
		inner = string(runes[:innerWidth])
		w = innerWidth
	}
	inner += strings.Repeat(" ", innerWidth-w)
	return "│" + inner + "│\r\n"
}

// headerLine builds a line with left and right text separated by spaces, filling
// exactly 78 inner display columns. Uses lipgloss.Width for ANSI-aware measurement.
func headerLine(left, right string) string {
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	spaces := innerWidth - leftW - rightW
	if spaces < 1 {
		spaces = 1
	}
	return "│" + left + strings.Repeat(" ", spaces) + right + "│\r\n"
}

// codeBox renders a framed code inside a 22-char-wide box, wrapped in outer │ borders.
// Each of the 3 returned lines is a full 80-char box row.
func codeBox(code string) string {
	// Center the raw code text (no ANSI) within 20 chars for the inner box.
	rawLen := lipgloss.Width(code)
	totalPad := 20 - rawLen
	if totalPad < 0 {
		totalPad = 0
	}
	leftPad := totalPad / 2
	rightPad := totalPad - leftPad
	inner := strings.Repeat(" ", leftPad) + codeStyle.Render(code) + strings.Repeat(" ", rightPad)
	return line("     ┌────────────────────┐") +
		line("     │" + inner + "│") +
		line("     └────────────────────┘")
}

// --- Shared Bubbletea helpers ---

// ctxQuitCmd returns a Cmd that fires QuitMsg when ctx is cancelled.
// Used by interactive models to respect SSH disconnect / deadline.
func ctxQuitCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		<-ctx.Done()
		return tea.QuitMsg{}
	}
}

// staticModel is a Bubbletea model that renders once and immediately quits.
// Used for all non-interactive views so they run inside a proper tea.Program.
type staticModel struct{ view string }

func (m staticModel) Init() tea.Cmd                           { return tea.Quit }
func (m staticModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }
func (m staticModel) View() string                            { return m.view }

// runStatic runs a one-shot Bubbletea program that renders content once and exits.
func runStatic(w io.Writer, content string) error {
	p := tea.NewProgram(staticModel{view: content},
		tea.WithOutput(w),
		tea.WithInput(strings.NewReader("")),
		tea.WithoutSignals(),
	)
	_, err := p.Run()
	return err
}

// --- HomeView ---

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
	label       string
	available   bool
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
	ctx    context.Context
	cursor int
	items  []homeItem
	action HomeAction
	notice string // transient "coming soon" message
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
		"       ███████╗██╗    ██╗██╗ ██████╗  ██████╗ ██╗   ██╗",
		"       ██╔════╝██║    ██║██║██╔════╝ ██╔════╝ ╚██╗ ██╔╝",
		"       ███████╗██║ █╗ ██║██║██║  ███╗██║  ███╗ ╚████╔╝",
		"       ╚════██║██║███╗██║██║██║   ██║██║   ██║  ╚██╔╝",
		"       ███████║╚███╔███╔╝██║╚██████╔╝╚██████╔╝   ██║",
		"       ╚══════╝ ╚══╝╚══╝ ╚═╝ ╚═════╝  ╚═════╝    ╚═╝",
	}

	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(headerLine(" swiggy.ssh", connStyle.Render("● Connected SSH ")))
	sb.WriteString(divider())
	sb.WriteString(line(""))
	for _, l := range logo {
		sb.WriteString(line(l))
	}
	sb.WriteString(line(""))
	sb.WriteString(line("                    Order groceries from your terminal"))
	sb.WriteString(line(""))
	sb.WriteString(divider())
	sb.WriteString(line(" What would you like to do?"))
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
	sb.WriteString(line(mutedStyle.Render(" ↑/↓ move   enter select   q quit")))
	sb.WriteString(bottom())
	return sb.String()
}

func (v HomeView) Render(ctx context.Context, w io.Writer) error {
	in := v.In
	if in == nil {
		in = strings.NewReader("")
	}
	m := homeModel{ctx: ctx, cursor: 0, items: homeItems}
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
	m := homeModel{ctx: ctx, cursor: 0, items: homeItems}
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

// --- LoginWaitingView ---

// LoginWaitingView renders the "open browser and enter code" prompt.
type LoginWaitingView struct {
	LoginURL string
	RawCode  string
}

func (v LoginWaitingView) Render(_ context.Context, w io.Writer) error {
	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(line(" swiggy.ssh > Login"))
	sb.WriteString(divider())
	sb.WriteString(line(""))
	sb.WriteString(line(" You need to connect your Swiggy account before placing orders."))
	sb.WriteString(line(""))
	sb.WriteString(line(" Open this URL in your browser:"))
	sb.WriteString(line(""))
	sb.WriteString(line(" " + accentStyle.Render(v.LoginURL)))
	sb.WriteString(line(""))
	sb.WriteString(line(" Enter code:"))
	sb.WriteString(line(""))
	sb.WriteString(codeBox(v.RawCode))
	sb.WriteString(line(""))
	sb.WriteString(line(" Waiting for login..."))
	sb.WriteString(line(""))
	sb.WriteString(line(" Status: " + mutedStyle.Render("not connected")))
	sb.WriteString(line(""))
	sb.WriteString(divider())
	sb.WriteString(line(mutedStyle.Render(" r refresh   c copy URL   b back   q quit")))
	sb.WriteString(bottom())
	return runStatic(w, sb.String())
}

// --- LoginSuccessView ---

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
	ctx     context.Context
	cursor  int
	choices []string
	name    string
	email   string
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
	sb.WriteString(line(" swiggy.ssh > Login"))
	sb.WriteString(divider())
	sb.WriteString(line(""))
	sb.WriteString(line(" " + successStyle.Render("✓ Swiggy account connected")))
	sb.WriteString(line(""))
	sb.WriteString(line(" Signed in as: " + boldStyle.Render(m.name) + "  <" + m.email + ">"))
	sb.WriteString(line(""))
	sb.WriteString(line(" You can now search Instamart, manage your cart, and place COD orders."))
	sb.WriteString(line(""))
	sb.WriteString(line(" Continue to:"))
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
	sb.WriteString(line(mutedStyle.Render(" ↑/↓ move   enter select   b back   q quit")))
	sb.WriteString(bottom())
	return sb.String()
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
		ctx:     ctx,
		cursor:  0,
		choices: loginSuccessChoices,
		name:    name,
		email:   email,
	}
	p := tea.NewProgram(m,
		tea.WithOutput(w),
		tea.WithInput(in),
		tea.WithoutSignals(),
	)
	_, err := p.Run()
	return err
}

// --- ReconnectView ---

// ReconnectView renders the re-auth prompt shown before a new login code.
// Inline (no full-screen box): shown mid-session when re-auth is needed.
type ReconnectView struct {
	RawCode string
}

func (v ReconnectView) Render(_ context.Context, w io.Writer) error {
	content := "\r\n" +
		"  Your session needs re-authentication.\r\n" +
		"  Enter this code in the browser login page:\r\n" +
		"\r\n" +
		"     " + accentStyle.Render(boldStyle.Render(v.RawCode)) + "\r\n" +
		"\r\n" +
		"  Waiting...\r\n"
	_, err := fmt.Fprint(w, content)
	return err
}

// --- AccountHomeView ---

// AccountHomeView renders the minimal account status for a returning user.
// Inline (no full-screen box): shown as a brief status block before the main menu.
type AccountHomeView struct {
	SSHFingerprint string
	Account        auth.OAuthAccount
}

func (v AccountHomeView) Render(_ context.Context, w io.Writer) error {
	status := v.Account.Status
	if status == "" {
		status = auth.OAuthAccountStatusActive
	}

	var statusRendered string
	switch status {
	case auth.OAuthAccountStatusActive:
		statusRendered = connStyle.Render(status)
	case auth.OAuthAccountStatusRevoked:
		statusRendered = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4444")).Render(status)
	default:
		statusRendered = accentStyle.Render(status)
	}

	content := "\r\n" +
		"  ┌────────────────────────────────────────┐\r\n" +
		"  │       swiggy.dev · Account             │\r\n" +
		"  └────────────────────────────────────────┘\r\n" +
		"\r\n" +
		"  SSH key : " + v.SSHFingerprint + "\r\n" +
		"  Provider: swiggy\r\n" +
		"  Status  : " + statusRendered + "\r\n"
	_, err := fmt.Fprint(w, content)
	return err
}

// --- InstamartPlaceholderView ---

// InstamartPlaceholderView is the backward-compat alias used by runSession.
// Renders the full Instamart screen with placeholder/zero values.
type InstamartPlaceholderView struct {
	UserID string
}

func (v InstamartPlaceholderView) Render(ctx context.Context, w io.Writer) error {
	return InstamartView{
		AddressLabel:  "Home",
		AddressLine:   "221B, 12th Main, Indiranagar, Bengaluru",
		CartItemCount: 0,
	}.Render(ctx, w)
}

// --- InstamartView ---

// InstamartView renders the full Instamart screen.
type InstamartView struct {
	AddressLabel  string // e.g. "Home"
	AddressLine   string // e.g. "221B, 12th Main, Indiranagar, Bengaluru"
	CartItemCount int
	In            io.Reader // if nil, static render
}

var instamartChoices = []string{
	"Search products",
	"Your go-to items",
	"View cart",
	"Track recent order",
	"Change address",
}

type instamartModel struct {
	ctx          context.Context
	cursor       int
	choices      []string
	addressLabel string
	addressLine  string
	cartCount    int
}

func (m instamartModel) Init() tea.Cmd {
	return ctxQuitCmd(m.ctx)
}

func (m instamartModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m instamartModel) View() string {
	deliverTo := "Delivering to " + m.addressLabel
	cartLine := fmt.Sprintf("Cart: %d items", m.cartCount)

	var sb strings.Builder
	sb.WriteString(top())
	sb.WriteString(headerLine(" swiggy.ssh > Instamart", deliverTo))
	sb.WriteString(divider())
	sb.WriteString(line(""))
	sb.WriteString(line(" " + boldStyle.Render("Instamart") + " — Groceries and daily essentials in minutes."))
	sb.WriteString(line(""))
	sb.WriteString(line(" Address: " + boldStyle.Render(m.addressLabel) + " — " + m.addressLine))
	sb.WriteString(line(" " + cartLine))
	sb.WriteString(line(""))
	sb.WriteString(divider())
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
	sb.WriteString(line(mutedStyle.Render(" / search   c cart   a address   b back   ? help")))
	sb.WriteString(bottom())
	return sb.String()
}

func (v InstamartView) Render(ctx context.Context, w io.Writer) error {
	in := v.In
	if in == nil {
		in = strings.NewReader("")
	}
	m := instamartModel{
		ctx:          ctx,
		cursor:       0,
		choices:      instamartChoices,
		addressLabel: v.AddressLabel,
		addressLine:  v.AddressLine,
		cartCount:    v.CartItemCount,
	}
	p := tea.NewProgram(m,
		tea.WithOutput(w),
		tea.WithInput(in),
		tea.WithoutSignals(),
	)
	_, err := p.Run()
	return err
}

// --- RevokedView ---

// RevokedView renders the account-revoked message.
type RevokedView struct{}

func (v RevokedView) Render(_ context.Context, w io.Writer) error {
	_, err := fmt.Fprint(w,
		"\r\n"+
			"  Your account access has been revoked.\r\n"+
			"  Please contact support.\r\n",
	)
	return err
}

// --- ErrorView ---

// ErrorView renders a generic recoverable error message.
type ErrorView struct {
	Message string
}

func (v ErrorView) Render(_ context.Context, w io.Writer) error {
	_, err := fmt.Fprintf(w, "\r\n  %s\r\n", v.Message)
	return err
}

// Compile-time assertions that all views implement View.
var (
	_ View = HomeView{}
	_ View = LoginWaitingView{}
	_ View = LoginSuccessView{}
	_ View = ReconnectView{}
	_ View = AccountHomeView{}
	_ View = InstamartPlaceholderView{}
	_ View = InstamartView{}
	_ View = RevokedView{}
	_ View = ErrorView{}
)
