package main

import (
	"fmt"
	"sort"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/gwillem/whip/internal/model"
)

const (
	padding      = "  "
	defaultWidth = 60
	colorA       = "#5A56E0"
	colorB       = "#EE6FF8"
)

var (
	// https://raw.githubusercontent.com/muesli/termenv/master/examples/color-chart/color-chart.png

	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("202")).Render
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("112")).Render
	blue   = lipgloss.NewStyle().Foreground(lipgloss.Color("27")).Render
	dark   = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Render
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Render

	BUSY  = blue("BUSY")
	DONE  = green("DONE")
	ERROR = red("ERROR")
	// DONE  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("112")).SetString("DONE").String()
	// ERROR = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("202")).SetString("ERROR").String()
)

type (
	// tickMsg time.Time
	bar struct {
		idx    int
		total  int
		status string
		perc   float64
		m      *progress.Model
	}
	tuiModel struct {
		bars map[model.TargetName]*bar
	}
)

func (m tuiModel) Init() tea.Cmd {
	return nil
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// fmt.Println("got update")
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m, tea.Quit

	case tea.WindowSizeMsg:
		// windowsizemsg sent only at start (unless resizing),
		// so need to determine bar width at creation time
		// m.screenWidth = msg.Width
		// todo, resize existing bars
		return m, nil

	case model.ReportMsg:
		tr := msg.TaskResult
		// fmt.Println("got task result", msg)
		perc := float64(msg.TaskIdx) / float64(msg.TaskTotal)

		b := m.bars[tr.Host]
		if b == nil {
			p := progress.New(
				progress.WithGradient(colorA, colorB),
				progress.WithWidth(defaultWidth))
			b = &bar{m: &p}
			m.bars[tr.Host] = b
		}

		b.perc = perc
		b.total = msg.TaskTotal
		b.idx = msg.TaskIdx

		if tr.Status != 0 {
			b.status = ERROR
		} else if perc == 1 {
			b.status = DONE
		} else {
			b.status = BUSY
		}

		return m, nil

	default:
		return m, nil
	}
}

func (m tuiModel) View() string {
	// fmt.Println("got view")
	var s string

	targets := []model.TargetName{}
	for t := range m.bars {
		targets = append(targets, t)
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i] < targets[j]
	})

	for _, t := range targets {
		bar := m.bars[t]
		counter := fmt.Sprintf("%d/%d", bar.idx, bar.total)

		s += fmt.Sprintf("%-5s %20.20s %s %s\n",
			counter, t, bar.m.ViewAs(bar.perc), bar.status)
	}
	return s
}

func createTui() *tea.Program {
	tui := tea.NewProgram(tuiModel{
		bars: map[model.TargetName]*bar{},
	})

	go func() {
		if _, err := tui.Run(); err != nil {
			log.Fatal(err)
		}
	}()
	return tui
}
