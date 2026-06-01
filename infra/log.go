package infra

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DailyRotateWriter is an io.WriteCloser that rolls to a new file each day
// and deletes files older than maxDays.
type DailyRotateWriter struct {
	dir     string
	base    string
	maxDays int

	mu   sync.Mutex
	file *os.File
	day  string
}

func NewDailyRotateWriter(dir, base string, maxDays int) *DailyRotateWriter {
	return &DailyRotateWriter{dir: dir, base: base, maxDays: maxDays}
}

func (w *DailyRotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if today != w.day {
		if w.file != nil {
			w.file.Close()
			w.file = nil
		}
		path := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.base, today))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return 0, err
		}
		w.file = f
		w.day = today
		w.cleanup()
	}
	return w.file.Write(p)
}

func (w *DailyRotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

func (w *DailyRotateWriter) cleanup() {
	pattern := filepath.Join(w.dir, w.base+"-*.log")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -w.maxDays)
	for _, path := range matches {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(path)
		}
	}
}
