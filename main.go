package main

import (
	"fmt"
	//"net"
	"os"
	//"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rdawson46/ping2/ui"
	//"github.com/rdawson46/ping2/ping"
)

func main(){
    p := tea.NewProgram(ui.InitializeModel(), tea.WithAltScreen())

    if _, err := p.Run(); err != nil {
        fmt.Println("broke:", err)
        os.Exit(1)
    }

    /*
    p := ping.NewPinger()

    ra, err := net.ResolveIPAddr("ip4:icmp", "www.google.com")

    if err != nil {
        fmt.Println("no addr")
        os.Exit(1)
    }

    p.AddIPAddr(ra)

    p.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
        fmt.Printf("IP Addr: %s recieve, RTT: %v\n", addr.String(), rtt)
    }

    p.OnIdle = func() {
        fmt.Printf("Finished")
    }

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

}
