package main

import (
	"fmt"
	"os"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rdawson46/ping2/ui"
)

func main(){
    p := tea.NewProgram(ui.InitializeModel(), tea.WithAltScreen())

    if _, err := p.Run(); err != nil {
        fmt.Println("broke:", err)
        os.Exit(1)
    }
}
