package ui

import (
    "os"
    "net"
    "time"
	//"math/rand"
	"github.com/guptarohit/asciigraph"
    tea "github.com/charmbracelet/bubbletea"
)

type timerMsg struct{}

type model struct {
    delays []float64
    timer  chan timerMsg
}

func InitializeModel() model {
    return model{
        timer: make(chan timerMsg),
        delays: make([]float64, 0),
    }
}

func (m model) Init() tea.Cmd {
    return tea.Batch(
        timer(m.timer),
        waitForTimer(m.timer),
    )
}

func (m model) View() string {
    if len(m.delays) == 0 {
        return "testing..."
    }

    graph := asciigraph.Plot(
        m.delays,
        asciigraph.Precision(2),
        asciigraph.SeriesColors(asciigraph.Green),
        asciigraph.Width(len(m.delays) - 1),
        asciigraph.Height(30),
    )
    return graph
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd){
    switch msg := msg.(type) {
    case timerMsg:
        s := time.Now()
        _, err := net.Dial("tcp", "www.google.com:80")

        if err != nil {os.Exit(1)}
        end := time.Now()

        total := end.Sub(s).Abs()

        //m.delays = append(m.delays, rand.Float64())
        m.delays = append(m.delays, float64(total.Abs().Milliseconds()))
        return m, waitForTimer(m.timer)

    case tea.KeyMsg:
        switch msg.String(){
        case "q", "ctrl+c":
            return m, tea.Quit
        }
    }

    return m, nil
}

func timer(sub chan<- timerMsg) tea.Cmd {
    return func() tea.Msg {
        for {
            time.Sleep(time.Second * 3)
            sub <- timerMsg{}
        }
    }
}

func waitForTimer(sub <-chan timerMsg) tea.Cmd {
    return func() tea.Msg {
        return <- sub
    }
}
