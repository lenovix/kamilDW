package downloader

// Downloader is an alias for Engine for backwards-compatibility.
type Downloader = Engine

// NewDownloader returns a new instance of Downloader/Engine.
func NewDownloader() *Downloader {
	return NewEngine()
}

// Probe delegates to Preflight on Engine.
func (e *Engine) Probe(url string, headers map[string]string) (total int64, acceptRanges bool, filename string, err error) {
	return e.Preflight(url, headers)
}
