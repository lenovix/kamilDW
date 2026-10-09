package engine

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/kkdai/youtube/v2"
)

var sanitizeRegexp = regexp.MustCompile(`[/\\?%*:|"<>]+`)

// ResolveURL resolves video page URLs (like YouTube) to direct media download URLs.
func ResolveURL(rawURL string) (directURL string, filename string, err error) {
	if strings.Contains(rawURL, "youtube.com") || strings.Contains(rawURL, "youtu.be") {
		client := youtube.Client{}
		video, err := client.GetVideo(rawURL)
		if err != nil {
			return "", "", fmt.Errorf("youtube resolve error: %w", err)
		}
		formats := video.Formats.WithAudioChannels()
		if len(formats) == 0 {
			return "", "", fmt.Errorf("no suitable audio/video format found")
		}
		formats.Sort()
		best := formats[0]

		safeTitle := sanitizeRegexp.ReplaceAllString(video.Title, "_")
		safeTitle = strings.TrimSpace(safeTitle)
		if safeTitle == "" {
			safeTitle = "youtube_video"
		}

		ext := ".mp4"
		if strings.Contains(best.MimeType, "webm") {
			ext = ".webm"
		}

		return best.URL, safeTitle + ext, nil
	}

	return rawURL, "", nil
}
