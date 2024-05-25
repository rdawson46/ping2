package ui

import (
	"math/rand"
	"time"
    "net"
    "os"
    "fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/guptarohit/asciigraph"
	"github.com/rdawson46/ping2/ping"
)

type timerMsg struct{}

type model struct {
    delays []float64
    timer  chan timerMsg
    pinger *ping.Pinger
}

func InitializeModel() model {
    // TODO: need to set values for pinger

    p := ping.NewPinger()

    // TODO: move most of this into the pinger module
    ra, err := net.ResolveIPAddr("ip4:icmp", "www.google.com")

    if err != nil {
        fmt.Println("no addr")
        os.Exit(1)
    }

    p.AddIPAddr(ra)

    // TODO: not really required ============================
    p.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
        fmt.Printf("IP Addr: %s recieve, RTT: %v\n", addr.String(), rtt)
    }

    p.OnIdle = func() {
        fmt.Printf("Finished")
    }
    // ======================================================

    // TODO: need to adapt this into a bubble tea way friendly way
    /*
    p.RunLoop()

    ticker := time.NewTicker(time.Second * 5)

    select {
    case <- p.Done():
        if err := p.Err(); err != nil {
            fmt.Println(err)
        }
    case <- ticker.C:
        break
    }

    ticker.Stop()
    p.Stop()
    */

    return model{
        timer: make(chan timerMsg),
        delays: make([]float64, 0),
        pinger: p,
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
        asciigraph.Width(len(m.delays)),
        asciigraph.Height(30),
    )
    return graph
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd){
    switch msg := msg.(type) {
    case timerMsg:
        //m.delays = append(m.delays, rand.Float64())
        //m.delays = append(m.delays, ping.Ping("www.google.com"))
        x := rand.Float64()
        m.delays = append(m.delays, x)
        return m, waitForTimer(m.timer)

    case tea.KeyMsg:
        switch msg.String() {
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
