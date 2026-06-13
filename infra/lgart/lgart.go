package lgart

import (
	"bytes"
	"compress/gzip"
	"io"
	"time"

	"gopkg.in/yaml.v3"
)

type File struct {
	Version     int            `yaml:"version"`
	GeneratedBy string         `yaml:"generated_by,omitempty"`
	ExportedAt  time.Time      `yaml:"exported_at"`
	Knowledge   []KnowledgeEntry `yaml:"knowledge,omitempty"`
	Nodes       []Node         `yaml:"nodes,omitempty"`
}

type KnowledgeEntry struct {
	Key     string   `yaml:"key"`
	Content string   `yaml:"content"`
	Labels  []string `yaml:"labels,omitempty"`
}

type Node struct {
	File        string   `yaml:"file"`
	Symbol      string   `yaml:"symbol"`
	Kind        string   `yaml:"kind"`
	Receiver    string   `yaml:"receiver,omitempty"`
	Description string   `yaml:"description"`
	ReturnType  string   `yaml:"return_type,omitempty"`
	DependsOn   []string `yaml:"depends_on,omitempty"`
}

type ImportResult struct {
	NodesImported     int `json:"nodes_imported"`
	NodesSkipped      int `json:"nodes_skipped"`
	KnowledgeImported int `json:"knowledge_imported"`
	KnowledgeSkipped  int `json:"knowledge_skipped"`
}

func Encode(f *File) ([]byte, error) {
	raw, err := yaml.Marshal(f)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func Decode(data []byte) (*File, error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		raw, err := io.ReadAll(gz)
		if err != nil {
			return nil, err
		}
		data = raw
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}
