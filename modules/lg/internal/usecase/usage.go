package usecase

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

func usageCachePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lemongrass", "usage-cache.json")
}

type usageCacheFile struct {
	Data UsageData `json:"data"`
	At   time.Time `json:"at"`
}

func loadUsageCacheFile() *usageCacheFile {
	data, err := os.ReadFile(usageCachePath())
	if err != nil {
		return nil
	}
	var cf usageCacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil
	}
	return &cf
}

func saveUsageCacheFile(d UsageData) {
	b, err := json.Marshal(usageCacheFile{Data: d, At: time.Now()})
	if err != nil {
		return
	}
	os.WriteFile(usageCachePath(), b, 0644)
}

var usageAnsiRe  = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b[()][A-Z0-9]|\x1b[=>]|\x1b[^\[]`)
var usagePctRe   = regexp.MustCompile(`(\d+)%\s*used`)
var usageResets  = regexp.MustCompile(`(?i)resets\s*(.+?)[\r\n]`)
var usageSessRe  = regexp.MustCompile(`(?i)current\s*session`)
var usageWeekRe  = regexp.MustCompile(`(?i)current\s*week`)

type UsageData struct {
	SessionPct    int    `json:"session_pct"`
	SessionResets string `json:"session_resets"`
	WeekPct       int    `json:"week_pct"`
	WeekResets    string `json:"week_resets"`
}

type usageCacheEntry struct {
	data UsageData
	at   time.Time
}

type usageProvider interface {
	FetchUsage() string
}

var usageCacheMu sync.Mutex
var usageCached  *usageCacheEntry

func (u *LgUsecase) SetUsageProvider(p usageProvider) {
	u.usage = p
}

const usageFileTTL = 2 * time.Hour // used only as stale hint in file cache reads

// StartUsageScheduler fetches usage immediately then every 5 minutes.
// The cache is always warm -- GetUsage never blocks on a PTY spawn.
func (u *LgUsecase) StartUsageScheduler(ctx context.Context) {
	go func() {
		// Initial fetch after a short delay so the server is fully up.
		time.Sleep(5 * time.Second)
		u.refreshUsageCache()

		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				u.refreshUsageCache()
			}
		}
	}()
}

func (u *LgUsecase) refreshUsageCache() {
	if u.usage == nil {
		return
	}
	raw := u.usage.FetchUsage()
	log.Printf("[usage] scheduled fetch raw_len=%d", len(raw))
	data := parseUsageOutput(raw)
	log.Printf("[usage] scheduled parsed session_pct=%d week_pct=%d", data.SessionPct, data.WeekPct)
	usageCacheMu.Lock()
	usageCached = &usageCacheEntry{data: data, at: time.Now()}
	usageCacheMu.Unlock()
	saveUsageCacheFile(data)
}

// GetUsage returns cached data only -- never triggers a live fetch.
func (u *LgUsecase) GetUsage(_ context.Context) UsageData {
	// In-memory first
	usageCacheMu.Lock()
	if usageCached != nil {
		data := usageCached.data
		usageCacheMu.Unlock()
		return data
	}
	usageCacheMu.Unlock()

	// File cache fallback (server restart before scheduler fires)
	if cf := loadUsageCacheFile(); cf != nil {
		usageCacheMu.Lock()
		usageCached = &usageCacheEntry{data: cf.Data, at: cf.At}
		usageCacheMu.Unlock()
		return cf.Data
	}

	return UsageData{}
}

func parseUsageOutput(raw string) UsageData {
	clean := usageAnsiRe.ReplaceAllString(raw, "")

	var data UsageData

	sessLoc := usageSessRe.FindStringIndex(clean)
	weekLoc := usageWeekRe.FindStringIndex(clean)

	var sessBlock, weekBlock string
	switch {
	case sessLoc != nil && weekLoc != nil && weekLoc[0] > sessLoc[0]:
		sessBlock = clean[sessLoc[0]:weekLoc[0]]
		weekBlock = clean[weekLoc[0]:]
	case sessLoc != nil:
		sessBlock = clean[sessLoc[0]:]
	case weekLoc != nil:
		weekBlock = clean[weekLoc[0]:]
	}

	if m := usagePctRe.FindStringSubmatch(sessBlock); len(m) > 1 {
		data.SessionPct, _ = strconv.Atoi(m[1])
	}
	if m := usageResets.FindStringSubmatch(sessBlock); len(m) > 1 {
		data.SessionResets = formatResetTime(strings.TrimSpace(m[1]))
	}
	if m := usagePctRe.FindStringSubmatch(weekBlock); len(m) > 1 {
		data.WeekPct, _ = strconv.Atoi(m[1])
	}
	if m := usageResets.FindStringSubmatch(weekBlock); len(m) > 1 {
		data.WeekResets = formatResetTime(strings.TrimSpace(m[1]))
	}

	return data
}

var (
	spaceBeforeParen  = regexp.MustCompile(`(\S)\(`)
	spaceLetterDigit  = regexp.MustCompile(`([a-zA-Z])(\d)`)
	spaceAfterComma   = regexp.MustCompile(`,(\S)`)
)

func formatResetTime(s string) string {
	s = spaceBeforeParen.ReplaceAllString(s, "$1 (")
	s = spaceLetterDigit.ReplaceAllString(s, "$1 $2")
	s = spaceAfterComma.ReplaceAllString(s, ", $1")
	return s
}
