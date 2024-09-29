package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mykso/myks/internal/myks"
	"github.com/mykso/myks/internal/prototypes"
	"github.com/pkg/errors"
)

var windowWidth int = 80

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type model struct {
	table table.Model
}

func (m model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		windowWidth = msg.Width
		cols := m.table.Columns()
		switch {
		case windowWidth == 0:
			break
		case windowWidth < 80:
			cols[0].Width = 30
			cols[1].Width = 10
			cols[2].Width = 0
			cols[3].Width = 0
			cols[4].Width = 0
			cols[5].Width = 0
		case windowWidth > 80:
			cols[0].Width = 30
			cols[1].Width = 10
			cols[2].Width = (windowWidth - 60) / 4
			cols[3].Width = (windowWidth - 60) / 4
			cols[4].Width = (windowWidth - 60) / 4
			cols[5].Width = (windowWidth - 60) / 4
		}
		m.table.SetColumns(cols)
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return baseStyle.Render(m.table.View()) + "\n  " + m.table.HelpView() + "\n"
}
func New(g *myks.Globe) error {
	columns := []table.Column{
		{Title: "Protoype", Width: 30},
		{Title: "sources", Width: 10},
		{Title: "helm", Width: 12},
		{Title: "static", Width: 12},
		{Title: "ytt", Width: 12},
		{Title: "ytt-pgk", Width: 12},
	}
	protos, err := prototypes.CollectPrototypes(g)
	if err != nil {
		return errors.Wrapf(err, "failed to collect prototypes")
	}
	rows := []table.Row{}
	for _, name := range protos {
		proto, err := prototypes.Load(g, name)
		if err != nil {
			return errors.Wrapf(err, "failed to load prototype %s", name)
		}
		var helm, static, ytt, yttPkg []string
		for _, source := range proto.Sources {
			switch source.Kind {
			case prototypes.Helm:
				helm = append(helm, source.Name)
			case prototypes.Static:
				static = append(static, source.Name)
			case prototypes.Ytt:
				ytt = append(ytt, source.Name)
			case prototypes.YttPkg:
				yttPkg = append(yttPkg, source.Name)
			}
		}
		row := table.Row{
			name,
			strconv.Itoa(len(proto.Sources)),
			strings.Join(helm, ", "),
			strings.Join(static, ", "),
			strings.Join(ytt, ", "),
			strings.Join(yttPkg, ", "),
		}
		rows = append(rows, row)
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m := model{t}
	if _, err := tea.NewProgram(&m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
	return nil
}
