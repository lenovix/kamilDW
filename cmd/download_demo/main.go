package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"kamildw/pkg/downloader"
)

func main() {
	// Target supporting Accept-Ranges: bytes
	testURL := "https://raw.githubusercontent.com/subhra74/xdm/master/README.md"
	outDir := filepath.Join(os.TempDir(), "kamildw_demo")
	_ = os.MkdirAll(outDir, 0o755)
	outFile := filepath.Join(outDir, "README.md")
	defer os.Remove(outFile)

	d := downloader.NewDownloader()

	fmt.Println("==> 1. Pre-flight Check...")
	total, ranges, fname, err := d.Probe(testURL, nil)
	if err != nil {
		fmt.Printf("Probe error: %v\n", err)
		return
	}
	fmt.Printf("File size     : %d bytes\n", total)
	fmt.Printf("Accept-Ranges : %v\n", ranges)
	fmt.Printf("Filename      : %s\n", fname)

	fmt.Println("\n==> 2. Starting Multi-part Download (4 workers)...")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err = d.Download(ctx, testURL, outFile, 4, nil, func(p downloader.ProgressEvent) {
		fmt.Printf("\rProgress: [%6.2f%%] %d / %d bytes | Speed: %.2f KB/s",
			p.Percentage, p.DownloadedBytes, p.TotalBytes, p.SpeedBytesPerSec/1024)
	})

	fmt.Println()
	if err != nil {
		fmt.Printf("Download failed: %v\n", err)
		return
	}

	fi, err := os.Stat(outFile)
	if err != nil {
		fmt.Printf("Stat error: %v\n", err)
		return
	}

	fmt.Printf("==> 3. Download Complete! File size on disk: %d bytes\n", fi.Size())
}
