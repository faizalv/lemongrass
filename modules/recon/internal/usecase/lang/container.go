package lang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/faizalv/lemongrass/modules/recon/entity"
)

type ContainerParser struct {
	serviceURL string
	priority   int
	detect     func(dir string) bool
}

func NewContainerParser(serviceURL string, priority int, detect func(string) bool) *ContainerParser {
	return &ContainerParser{serviceURL: serviceURL, priority: priority, detect: detect}
}

func (p *ContainerParser) Name() string           { return p.serviceURL }
func (p *ContainerParser) Priority() int          { return p.priority }
func (p *ContainerParser) Detect(dir string) bool { return p.detect(dir) }

func (p *ContainerParser) Parse(dir string, ig Ignorer) (*entity.ParseResult, error) {
	return p.call(dir, ig.Patterns())
}

// ParseFiles falls back to full-directory parsing -- lg-lang has no incremental endpoint.
func (p *ContainerParser) ParseFiles(dir string, ig Ignorer, _ []string) (*entity.ParseResult, error) {
	return p.call(dir, ig.Patterns())
}

func (p *ContainerParser) call(dir string, patterns []string) (*entity.ParseResult, error) {
	if patterns == nil {
		patterns = []string{}
	}
	body, _ := json.Marshal(map[string]any{
		"project_path":    dir,
		"ignore_patterns": patterns,
	})
	resp, err := http.Post(p.serviceURL+"/parse", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("lg-lang unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lg-lang: status %d", resp.StatusCode)
	}
	var result entity.ParseResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("lg-lang: bad JSON: %w", err)
	}
	return &result, nil
}
