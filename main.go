package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"kamildw/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func initLogger() (*os.File, string) {
	home, _ := os.UserHomeDir()
	appDir := filepath.Join(home, ".kamildw")
	_ = os.MkdirAll(appDir, 0o755)

	logPath := filepath.Join(appDir, "kamildw.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, ""
	}

	mw := io.MultiWriter(os.Stdout, f)
	log.SetOutput(mw)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	return f, logPath
}

func main() {
	webMode := flag.Bool("web", false, "Jalankan dalam mode Web Browser (tanpa GUI desktop)")
	flag.Parse()

	logFile, logPath := initLogger()
	if logFile != nil {
		defer logFile.Close()
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("[FATAL CRASH] Panic terdeteksi: %v\nStack trace:\n%s", r, string(debug.Stack()))
			fmt.Fprintf(os.Stderr, "Fatal Error! Cek log di: %s\n", logPath)
		}
	}()

	log.Println("==================================================")
	log.Println("[STARTUP] Memulai inisialisasi kamilDW...")
	log.Printf("[INFO] Lokasi file log: %s", logPath)

	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("[ERROR] Gagal membaca home directory user: %v", err)
	}

	appDir := filepath.Join(home, ".kamildw")
	_ = os.MkdirAll(appDir, 0o755)

	dbPath := filepath.Join(appDir, "kamildw.db")
	downloadDir := filepath.Join(home, "Downloads", "kamilDW")
	log.Printf("[INFO] Database SQLite: %s", dbPath)
	log.Printf("[INFO] Direktori Downloads: %s", downloadDir)

	srv, err := app.NewService(dbPath, downloadDir)
	if err != nil {
		log.Fatalf("[ERROR] Gagal inisialisasi service: %v", err)
	}
	log.Println("[OK] Service core & SQLite database berhasil diinisialisasi")

	// Set embedded assets untuk web dashboard
	distFS, err := fs.Sub(assets, "frontend/dist")
	if err == nil {
		srv.SetAssetFS(http.FS(distFS))
	}

	if err := srv.StartServer(); err != nil {
		log.Printf("[WARN] Server web/ekstensi gagal dijalankan: %v", err)
	} else {
		log.Println("[OK] Web Platform berjalan pada http://127.0.0.1:19890")
		fmt.Println(">> Web Platform siap diakses di: http://127.0.0.1:19890")
	}

	// Jika flag -web digunakan, jalankan web-only mode (ideal untuk browser development)
	if *webMode {
		log.Println("[MODE] Berjalan dalam Web Platform Mode (Headless). Tekan Ctrl+C untuk keluar.")
		fmt.Println(">> Berjalan dalam Web Platform Mode. Buka browser: http://127.0.0.1:19890")
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		log.Println("[EXIT] Web server dimatikan.")
		return
	}

	log.Println("[INFO] Meluncurkan jendela desktop Wails...")
	err = wails.Run(&options.App{
		Title:     "kamilDW - Download Manager",
		Width:     1280,
		Height:    800,
		MinWidth:  960,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 2, G: 6, B: 24, A: 1},
		Windows: &windows.Options{
			Theme: windows.Dark,
		},
		OnStartup: func(ctx context.Context) {
			log.Println("[OK] Aplikasi desktop berhasil berjalan (OnStartup)")
		},
		OnDomReady: func(ctx context.Context) {
			log.Println("[OK] UI React berhasil dimuat ke WebView (OnDomReady)")
		},
		OnShutdown: func(ctx context.Context) {
			log.Printf("[INFO] Aplikasi kamilDW ditutup pada %s", time.Now().Format(time.RFC3339))
		},
	})

	if err != nil {
		log.Fatalf("[FATAL] Gagal membuka antarmuka desktop Wails: %v", err)
	}
	log.Println("[EXIT] Aplikasi selesai dieksekusi dengan normal.")
}
