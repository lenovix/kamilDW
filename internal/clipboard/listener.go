package clipboard

import (
	"context"
	"net/url"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	openClipboard    = user32.NewProc("OpenClipboard")
	closeClipboard   = user32.NewProc("CloseClipboard")
	getClipboardData = user32.NewProc("GetClipboardData")
	globalLock       = kernel32.NewProc("GlobalLock")
	globalUnlock     = kernel32.NewProc("GlobalUnlock")
)

const cfUnicodeText = 13

type Listener struct {
	OnURL func(u string)
}

func New(onURL func(u string)) *Listener {
	return &Listener{OnURL: onURL}
}

func (l *Listener) Start(ctx context.Context) error {
	go func() {
		var lastText string
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				text, err := readClipboard()
				if err != nil || text == "" || text == lastText {
					continue
				}
				lastText = text
				if isValidURL(text) && l.OnURL != nil {
					l.OnURL(text)
				}
			}
		}
	}()
	return nil
}

func readClipboard() (string, error) {
	r1, _, _ := openClipboard.Call(0)
	if r1 == 0 {
		return "", nil
	}
	defer closeClipboard.Call()

	hMem, _, _ := getClipboardData.Call(cfUnicodeText)
	if hMem == 0 {
		return "", nil
	}

	ptr, _, _ := globalLock.Call(hMem)
	if ptr == 0 {
		return "", nil
	}
	defer globalUnlock.Call(hMem)

	// UTF-16 string scan
	var u16 []uint16
	for p := (*uint16)(unsafe.Pointer(ptr)); *p != 0; p = (*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 2)) {
		u16 = append(u16, *p)
	}
	return strings.TrimSpace(syscall.UTF16ToString(u16)), nil
}

func isValidURL(str string) bool {
	if !strings.HasPrefix(str, "http://") && !strings.HasPrefix(str, "https://") {
		return false
	}
	u, err := url.ParseRequestURI(str)
	return err == nil && u.Host != ""
}
