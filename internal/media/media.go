package media

import (
	"context"
	"fmt"
	"os/exec"
)

// HLS2MP4 converts an m3u8 file or URL to mp4 using ffmpeg.
// FFmpeg must be installed and available in PATH or provided via ffmpegPath.
func HLS2MP4(ctx context.Context, input string, output string, ffmpegPath string) error {
	bin := "ffmpeg"
	if ffmpegPath != "" {
		bin = ffmpegPath
	}

	// -y : overwrite output
	// -i : input url/file
	// -c copy : stream copy without re-encoding
	cmd := exec.CommandContext(ctx, bin, "-y", "-i", input, "-c", "copy", output)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg hls merge failed: %v\noutput: %s", err, string(out))
	}
	return nil
}
