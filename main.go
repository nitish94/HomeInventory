package main

import (
	"fmt"
	"homeinventory/db"
	"homeinventory/tui"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if _, err := os.Stat("inventory.db"); os.IsNotExist(err) {
		file, err := os.Create("inventory.db")
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
	}

	database, err := db.InitDB("inventory.db")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		log.Fatal(err)
	}

	p := tea.NewProgram(tui.NewModel(database), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
