package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"kamildw/internal/app"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("failed to resolve user home: %v", err)
	}

	appDir := filepath.Join(home, ".kamildw")
	_ = os.MkdirAll(appDir, 0o755)

	dbPath := filepath.Join(appDir, "kamildw.db")
	downloadDir := filepath.Join(home, "Downloads", "kamilDW")

	srv, err := app.NewService(dbPath, downloadDir)
	if err != nil {
		log.Fatalf("failed to initialize service: %v", err)
	}

	if err := srv.StartServer(); err != nil {
		log.Printf("extension server warning: %v", err)
	}

	fmt.Println("kamilDW Core Engine started on 127.0.0.1:19890")
	fmt.Printf("Database: %s\nDownloads: %s\n", dbPath, downloadDir)
}
