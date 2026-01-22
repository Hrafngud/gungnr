package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gungnr-cli/internal/cli/app"
	"gungnr-cli/internal/cli/prompts"
)

type UI struct {
	programMu sync.Mutex
	program   *tea.Program
	logo      string
}

func New(logo string) *UI {
	return &UI{logo: logo}
}

func (u *UI) Run(ctx context.Context, runner func(context.Context) error) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	model := newModel(u.logo)
	program := tea.NewProgram(model, tea.WithAltScreen())
	u.setProgram(program)

	runErrCh := make(chan error, 1)
	go func() {
		err := runner(runCtx)
		program.Send(runFinishedMsg{err: err})
		runErrCh <- err
	}()

	if err := program.Start(); err != nil {
		cancel()
		return err
	}

	cancel()
	return <-runErrCh
}

func (u *UI) SetSteps(steps []app.StepInfo) {
	u.send(setStepsMsg{steps: steps})
}

func (u *UI) StepStart(id string) {
	u.send(stepStartMsg{id: id})
}

func (u *UI) StepProgress(id, message string) {
	u.send(stepProgressMsg{id: id, message: message})
}

func (u *UI) StepDone(id, message string) {
	u.send(stepDoneMsg{id: id, message: message})
}

func (u *UI) StepError(id string, err error) {
	u.send(stepErrorMsg{id: id, err: err})
}

func (u *UI) Info(message string) {
	u.send(infoMsg{message: message})
}

func (u *UI) Warn(message string) {
	u.send(infoMsg{message: message})
}

func (u *UI) Prompt(ctx context.Context, prompt prompts.Prompt) (string, error) {
	response := make(chan promptResponse, 1)
	u.send(promptMsg{prompt: prompt, response: response})
	select {
	case resp := <-response:
		return resp.value, resp.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (u *UI) FinalSummary(summary app.Summary) {
	u.send(summaryMsg{summary: summary})
}

func (u *UI) send(msg tea.Msg) {
	u.programMu.Lock()
	program := u.program
	u.programMu.Unlock()
	if program == nil {
		return
	}
	program.Send(msg)
}

func (u *UI) setProgram(program *tea.Program) {
	u.programMu.Lock()
	defer u.programMu.Unlock()
	u.program = program
}

type stepStatus int

const (
	stepPending stepStatus = iota
	stepRunning
	stepDone
	stepError
)

const logoSpacing = 2

type stepItem struct {
	ID      string
	Title   string
	Status  stepStatus
	Message string
}

type promptState struct {
	active   bool
	prompt   prompts.Prompt
	input    textinput.Model
	response chan promptResponse
	errorMsg string
}

type model struct {
	steps        []stepItem
	stepIdx      map[string]int
	spinner      spinner.Model
	progress     progress.Model
	viewport     viewport.Model
	prompt       promptState
	logs         []string
	width        int
	height       int
	contentWidth int
	showLogo     bool
	logo         string
	logoWidth    int
	logoHeight   int
	done         bool
	err          error
	summary      *app.Summary
}

type setStepsMsg struct{ steps []app.StepInfo }

type stepStartMsg struct{ id string }

type stepProgressMsg struct {
	id      string
	message string
}

type stepDoneMsg struct {
	id      string
	message string
}

type stepErrorMsg struct {
	id  string
	err error
}

type infoMsg struct{ message string }

type promptMsg struct {
	prompt   prompts.Prompt
	response chan promptResponse
}

type promptResponse struct {
	value string
	err   error
}

type runFinishedMsg struct{ err error }

type summaryMsg struct{ summary app.Summary }

func newModel(logo string) model {
	spin := spinner.New()
	spin.Spinner = spinner.Line
	prog := progress.New(progress.WithDefaultGradient())
	vp := viewport.New(0, 0)
	logoText, logoWidth, logoHeight := normalizeLogo(logo)
	return model{
		steps:      []stepItem{},
		stepIdx:    map[string]int{},
		spinner:    spin,
		progress:   prog,
		viewport:   vp,
		logs:       []string{},
		logo:       logoText,
		logoWidth:  logoWidth,
		logoHeight: logoHeight,
	}
}

func (m model) Init() tea.Cmd {
	return spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		if m.prompt.active {
			switch msg.Type {
			case tea.KeyEnter:
				value := m.prompt.input.Value()
				resolved, err := prompts.Apply(m.prompt.prompt, value)
				if err != nil {
					m.prompt.errorMsg = err.Error()
					return m, nil
				}
				m.prompt.active = false
				m.prompt.errorMsg = ""
				m.prompt.response <- promptResponse{value: resolved}
				m.prompt.response = nil
				return m, nil
			case tea.KeyEsc:
				m.prompt.input.SetValue("")
				return m, nil
			default:
				var cmd tea.Cmd
				m.prompt.input, cmd = m.prompt.input.Update(msg)
				return m, cmd
			}
		}
		if m.done {
			if msg.String() == "q" || msg.Type == tea.KeyEnter {
				return m, tea.Quit
			}
		}
		if isScrollKey(msg) {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case tea.WindowSizeMsg:
		m.updateLayout(msg.Width, msg.Height)
	case setStepsMsg:
		m.steps = make([]stepItem, len(msg.steps))
		m.stepIdx = map[string]int{}
		for i, step := range msg.steps {
			m.steps[i] = stepItem{ID: step.ID, Title: step.Title, Status: stepPending}
			m.stepIdx[step.ID] = i
		}
		m.updateLayout(m.width, m.height)
		m.updateProgress()
	case stepStartMsg:
		m.setStatus(msg.id, stepRunning)
	case stepProgressMsg:
		m.setMessage(msg.id, msg.message)
	case stepDoneMsg:
		m.setStatus(msg.id, stepDone)
		m.setMessage(msg.id, msg.message)
	case stepErrorMsg:
		m.setStatus(msg.id, stepError)
		m.err = msg.err
		m.done = true
		m.appendLog(fmt.Sprintf("Error: %v", msg.err))
	case infoMsg:
		m.appendLog(msg.message)
	case promptMsg:
		m.prompt = promptState{active: true, prompt: msg.prompt, response: msg.response}
		m.prompt.input = textinput.New()
		m.prompt.input.Focus()
		m.prompt.input.Prompt = ""
		m.prompt.input.Placeholder = msg.prompt.Default
		if msg.prompt.Secret {
			m.prompt.input.EchoMode = textinput.EchoPassword
			m.prompt.input.EchoCharacter = '*'
		}
	case runFinishedMsg:
		m.done = true
		m.err = msg.err
		if msg.err != nil {
			m.appendLog(fmt.Sprintf("Bootstrap failed: %v", msg.err))
		}
	case summaryMsg:
		m.summary = &msg.summary
		m.appendSummary(msg.summary)
	}

	if m.prompt.active {
		var cmd tea.Cmd
		m.prompt.input, cmd = m.prompt.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	return m.renderLayout()
}

func (m model) renderLayout() string {
	var builder strings.Builder
	if m.showLogo && m.logo != "" {
		builder.WriteString(m.renderLogoTop())
		builder.WriteString("\n\n")
	}
	builder.WriteString(titleStyle.Render("Gungnr Bootstrap"))
	builder.WriteString("\n\n")

	builder.WriteString(m.renderProgress())
	builder.WriteString("\n")
	builder.WriteString(m.renderSteps())
	builder.WriteString("\n")

	builder.WriteString(m.renderBody())

	builder.WriteString("\n")
	builder.WriteString(m.renderFooter())
	return builder.String()
}

func (m *model) appendLog(message string) {
	m.logs = append(m.logs, message)
	if len(m.logs) > 200 {
		m.logs = m.logs[len(m.logs)-200:]
	}
	m.viewport.SetContent(strings.Join(m.logs, "\n"))
	m.viewport.GotoBottom()
}

func (m *model) appendSummary(summary app.Summary) {
	m.appendLog("Bootstrap configuration written.")
	m.appendLog("- Data directory: " + summary.DataDir)
	m.appendLog("- Templates directory: " + summary.TemplatesDir)
	m.appendLog("- State directory: " + summary.StateDir)
	m.appendLog("- .env path: " + summary.EnvPath)
	m.appendLog("- Panel hostname: " + summary.PanelURL)
	m.appendLog("- Cloudflared config: " + summary.CloudflaredConfig)
	m.appendLog("- Cloudflared log: " + summary.CloudflaredLog)
	m.appendLog("- Docker build log: " + summary.ComposeLog)
	m.appendLog("- Cloudflare tunnel: " + summary.CloudflaredTunnel + " (" + summary.CloudflaredTunnelID + ")")
}

func (m *model) setStatus(id string, status stepStatus) {
	idx, ok := m.stepIdx[id]
	if !ok {
		return
	}
	m.steps[idx].Status = status
	m.updateProgress()
}

func (m *model) setMessage(id, message string) {
	idx, ok := m.stepIdx[id]
	if !ok {
		return
	}
	m.steps[idx].Message = message
}

func (m *model) updateProgress() {
	if len(m.steps) == 0 {
		return
	}
	doneCount := 0
	for _, step := range m.steps {
		if step.Status == stepDone {
			doneCount++
		}
	}
	percent := float64(doneCount) / float64(len(m.steps))
	m.progress.SetPercent(percent)
}

func (m model) renderProgress() string {
	return m.progress.View()
}

func (m model) renderSteps() string {
	lines := make([]string, 0, len(m.steps))
	for _, step := range m.steps {
		prefix, style := statusLabel(step.Status)
		if step.Status == stepRunning {
			prefix = m.spinner.View() + " " + prefix
		}
		line := fmt.Sprintf("%s %s", prefix, step.Title)
		if step.Status == stepRunning && step.Message != "" {
			line = fmt.Sprintf("%s %s - %s", prefix, step.Title, step.Message)
		}
		lines = append(lines, style.Render(line))
	}
	return strings.Join(lines, "\n")
}

func (m model) renderBody() string {
	if m.prompt.active {
		return m.renderPrompt()
	}
	return m.renderLogs()
}

func (m model) renderLogs() string {
	return logBoxStyle.Render(m.viewport.View())
}

func (m model) renderPrompt() string {
	lines := []string{promptTitleStyle.Render("Input required")}
	lines = append(lines, promptLabelStyle.Render(m.prompt.prompt.Label))
	for _, line := range m.prompt.prompt.Help {
		lines = append(lines, line)
	}
	if m.prompt.prompt.Default != "" {
		lines = append(lines, fmt.Sprintf("Default: %s", m.prompt.prompt.Default))
	}
	lines = append(lines, m.prompt.input.View())
	if m.prompt.errorMsg != "" {
		lines = append(lines, errorStyle.Render(m.prompt.errorMsg))
	}
	style := promptBoxStyle
	if m.contentWidth > 0 {
		style = style.Width(m.contentWidth)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func (m model) renderLogoTop() string {
	if m.logo == "" || m.logoWidth == 0 {
		return ""
	}
	centered := m.logo
	if m.width > 0 {
		centered = lipgloss.PlaceHorizontal(m.width, lipgloss.Center, m.logo)
	}
	return logoStyle.Render(centered)
}

func (m model) renderFooter() string {
	if m.prompt.active {
		return footerStyle.Render("Enter to submit | Esc to clear | Quit: Ctrl+C")
	}
	if m.done {
		if m.err != nil {
			return footerStyle.Render("Bootstrap failed. Scroll: Up/Down/PageUp/PageDown | Quit: q")
		}
		return footerStyle.Render("Bootstrap complete. Scroll: Up/Down/PageUp/PageDown | Quit: q")
	}
	return footerStyle.Render("Running... Scroll: Up/Down/PageUp/PageDown | Quit: Ctrl+C")
}

func statusLabel(status stepStatus) (string, lipgloss.Style) {
	switch status {
	case stepRunning:
		return "[*]", runningStyle
	case stepDone:
		return "[+]", doneStyle
	case stepError:
		return "[x]", errorStyle
	default:
		return "[ ]", pendingStyle
	}
}

func (m *model) updateLayout(width, height int) {
	if width > 0 {
		m.width = width
	}
	if height > 0 {
		m.height = height
	}
	if m.width == 0 || m.height == 0 {
		return
	}
	m.contentWidth = m.width

	baseHeaderHeight := 3 + len(m.steps)
	footerHeight := 1
	minBodyHeight := 3
	headerHeight := baseHeaderHeight

	m.showLogo = false
	if m.logo != "" && m.logoWidth > 0 && m.width >= m.logoWidth {
		logoBlock := m.logoHeight + logoSpacing
		if m.height >= baseHeaderHeight+footerHeight+minBodyHeight+logoBlock {
			m.showLogo = true
			headerHeight += logoBlock
		}
	}

	m.viewport.Width = max(20, m.contentWidth-4)
	m.progress.Width = max(20, m.contentWidth-4)

	viewportHeight := m.height - headerHeight - footerHeight - 2
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	m.viewport.Height = viewportHeight
}

func normalizeLogo(raw string) (string, int, int) {
	trimmed := strings.TrimRight(raw, "\n")
	if trimmed == "" {
		return "", 0, 0
	}
	maxWidth := 0
	lines := strings.Split(trimmed, "\n")
	for _, line := range lines {
		if width := lipgloss.Width(line); width > maxWidth {
			maxWidth = width
		}
	}
	return trimmed, maxWidth, len(lines)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func isScrollKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
		return true
	default:
		return false
	}
}

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	pendingStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	runningStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	doneStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("70"))
	errorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	logBoxStyle      = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	promptBoxStyle   = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(1, 2)
	promptTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	promptLabelStyle = lipgloss.NewStyle().Bold(true)
	logoStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	footerStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)
