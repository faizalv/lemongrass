package usecase

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/faizalv/lemongrass/infra/lgart"
	"github.com/faizalv/lemongrass/modules/recon/entity"
)

func (u *ReconUsecase) SaveKnowledge(ctx context.Context, projectID int64, key, content string, labels []string) (bool, error) {
	vec, embedErr := u.embed.Embed(ctx, content)
	err := u.repo.SaveKnowledge(ctx, projectID, key, content, vec, labels)
	return embedErr == nil && len(vec) > 0, err
}

func (u *ReconUsecase) ReadKnowledge(ctx context.Context, projectID int64, key string) (string, error) {
	return u.repo.ReadKnowledge(ctx, projectID, key)
}

func (u *ReconUsecase) SearchKnowledge(ctx context.Context, projectID int64, query, label string) ([]entity.KnowledgeEntry, bool, error) {
	vec, embedErr := u.embed.Embed(ctx, query)
	if embedErr == nil {
		var results []entity.KnowledgeEntry
		var err error
		if label != "" {
			results, err = u.repo.SearchKnowledgeByLabel(ctx, projectID, label, vec, 5)
		} else {
			results, err = u.repo.SearchKnowledge(ctx, projectID, vec, 5)
		}
		if err == nil && len(results) > 0 {
			return results, false, nil
		}
	}
	// Fallback: substring match on key and content when embed is unavailable or entries have null embeddings.
	all, err := u.repo.ListKnowledge(ctx, projectID)
	if err != nil {
		return nil, true, err
	}
	queryLower := strings.ToLower(query)
	var out []entity.KnowledgeEntry
	for _, e := range all {
		if strings.Contains(strings.ToLower(e.Key), queryLower) ||
			strings.Contains(strings.ToLower(e.Content), queryLower) {
			out = append(out, e)
		}
	}
	if len(out) > 5 {
		out = out[:5]
	}
	return out, true, nil
}

func (u *ReconUsecase) ListKnowledge(ctx context.Context, projectID int64) ([]entity.KnowledgeEntry, error) {
	return u.repo.ListKnowledge(ctx, projectID)
}

func (u *ReconUsecase) DeleteKnowledge(ctx context.Context, projectID int64, key string) (bool, error) {
	return u.repo.DeleteKnowledge(ctx, projectID, key)
}

func (u *ReconUsecase) FindSimilarKnowledge(ctx context.Context, projectID int64, content, excludeKey string) ([]string, error) {
	vec, err := u.embed.Embed(ctx, content)
	if err != nil {
		return nil, nil
	}
	return u.repo.FindSimilarKnowledge(ctx, projectID, excludeKey, vec)
}

func (u *ReconUsecase) UpsertLabel(ctx context.Context, projectID int64, label, content string) error {
	vec, _ := u.embed.Embed(ctx, label+": "+content)
	return u.repo.UpsertLabel(ctx, projectID, label, vec)
}

func (u *ReconUsecase) FindSimilarLabels(ctx context.Context, projectID int64, label, content string) ([]string, error) {
	vec, err := u.embed.Embed(ctx, label+": "+content)
	if err != nil {
		return nil, nil
	}
	return u.repo.FindSimilarLabels(ctx, projectID, label, vec)
}

func (u *ReconUsecase) ListAllLabels(ctx context.Context, projectID int64) ([]string, error) {
	return u.repo.ListAllLabels(ctx, projectID)
}

func (u *ReconUsecase) SearchLabels(ctx context.Context, projectID int64, query string) ([]string, error) {
	vec, err := u.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return u.repo.SearchLabels(ctx, projectID, vec)
}

func (u *ReconUsecase) SearchKnowledgeByLabel(ctx context.Context, projectID int64, label, query string) ([]entity.KnowledgeEntry, error) {
	vec, err := u.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return u.repo.SearchKnowledgeByLabel(ctx, projectID, label, vec, 5)
}

func (u *ReconUsecase) NodeOverlap(ctx context.Context, projectID int64, keys []string) (matched, total int, ready bool, err error) {
	has, err := u.repo.HasNodes(ctx, projectID)
	if err != nil {
		return 0, 0, false, err
	}
	if !has {
		return 0, len(keys), false, nil
	}
	total = len(keys)
	matched, err = u.repo.CheckNodeOverlap(ctx, projectID, keys)
	return matched, total, true, err
}

func (u *ReconUsecase) ExportArtifacts(ctx context.Context, projectID int64, originID, projectLabel, gitOrigin, gitUser string) (*lgart.File, error) {
	nodes, err := u.repo.ListNodes(ctx, projectID, "", "", "")
	if err != nil {
		return nil, err
	}
	knowledge, err := u.repo.ListKnowledge(ctx, projectID)
	if err != nil {
		return nil, err
	}

	f := &lgart.File{
		Version:      2,
		GeneratedBy:  originID,
		ExportedAt:   time.Now().UTC(),
		ProjectLabel: projectLabel,
		GitOrigin:    gitOrigin,
		GitUser:      gitUser,
		EmbedModel:   u.embed.Model(ctx),
	}

	for _, n := range nodes {
		if n.Description == "" {
			continue
		}
		node := lgart.Node{
			File:        n.FilePath,
			Symbol:      n.Symbol,
			Kind:        n.Kind,
			Receiver:    n.Receiver,
			Description: n.Description,
			ReturnType:  n.ReturnType,
			DependsOn:   n.Calls,
		}
		if n.ContentHash != "" {
			node.ContentHash = n.ContentHash
			node.Branches = n.Branches
		}
		f.Nodes = append(f.Nodes, node)
	}

	for _, k := range knowledge {
		f.Knowledge = append(f.Knowledge, lgart.KnowledgeEntry{
			Key:     k.Key,
			Content: k.Content,
			Labels:  k.Labels,
		})
	}

	return f, nil
}

func (u *ReconUsecase) ImportArtifacts(ctx context.Context, projectID int64, f *lgart.File, force bool) (lgart.ImportResult, error) {
	ready, err := u.repo.HasNodes(ctx, projectID)
	if err != nil {
		return lgart.ImportResult{}, err
	}
	if !ready {
		return lgart.ImportResult{}, fmt.Errorf("semantic map not ready")
	}

	var result lgart.ImportResult
	localModel := u.embed.Model(ctx)

	for _, n := range f.Nodes {
		if n.ContentHash != "" {
			u.importV2Node(ctx, projectID, f, n, &result, force, localModel)
		} else {
			u.importV1Node(ctx, projectID, n, &result, force)
		}
	}

	for _, k := range f.Knowledge {
		if !force {
			existing, err := u.repo.ReadKnowledge(ctx, projectID, k.Key)
			if err == nil && existing != "" {
				result.KnowledgeSkipped++
				continue
			}
		}
		labels := k.Labels
		if labels == nil {
			labels = []string{}
		}
		if _, err := u.SaveKnowledge(ctx, projectID, k.Key, k.Content, labels); err == nil {
			result.KnowledgeImported++
		}
	}

	return result, nil
}

func (u *ReconUsecase) importV2Node(ctx context.Context, projectID int64, f *lgart.File, n lgart.Node, result *lgart.ImportResult, force bool, localModel string) {
	existing, err := u.repo.GetNodeByHash(ctx, projectID, n.File, n.Symbol, n.Kind, n.ContentHash)

	if err != nil {
		// Not found -- INSERT with placeholder structurals
		lang := langFromExt(n.File)
		branches := n.Branches
		if len(branches) == 0 {
			branches = []string{}
		}
		if err2 := u.repo.InsertLgartNode(ctx, projectID, n.File, n.Symbol, n.Kind, lang, n.ContentHash, n.Description, n.ReturnType, n.DependsOn, branches); err2 != nil {
			return
		}
		result.NodesInserted++
		u.triggerEmbed(ctx, projectID, n, f.EmbedModel, localModel, true)
		return
	}

	if existing.Status == "explored" && !force {
		result.NodesSkipped++
		return
	}

	// Annotate in place (unexplored, or explored+force)
	rows, err := u.repo.AnnotateNodeByHash(ctx, projectID, n.File, n.Symbol, n.Kind, n.ContentHash, n.Description, n.ReturnType, n.DependsOn)
	if err != nil || rows == 0 {
		return
	}
	result.NodesImported++
	u.triggerEmbed(ctx, projectID, n, f.EmbedModel, localModel, false)
}

func (u *ReconUsecase) importV1Node(ctx context.Context, projectID int64, n lgart.Node, result *lgart.ImportResult, force bool) {
	all, err := u.repo.FindNodesBySymbol(ctx, projectID, n.File, n.Symbol)
	if err != nil || len(all) != 1 {
		// Ambiguous or not found -- skip
		return
	}
	existing := all[0]
	if existing.Status == "explored" && !force {
		result.NodesSkipped++
		return
	}
	rows, err := u.repo.AnnotateNode(ctx, projectID, n.File, n.Symbol, n.Kind, n.Description, n.ReturnType, n.DependsOn)
	if err != nil || rows == 0 {
		return
	}
	result.NodesImported++
	u.annotating.Add(1)
	go func(filePath, symbol, desc string) {
		defer u.annotating.Add(-1)
		vec, err := u.embed.Embed(context.Background(), desc)
		if err != nil {
			return
		}
		u.repo.SetEmbedding(context.Background(), projectID, filePath, symbol, vec)
	}(n.File, n.Symbol, n.Description)
}

func (u *ReconUsecase) triggerEmbed(ctx context.Context, projectID int64, n lgart.Node, fileModel, localModel string, isInsert bool) {
	if fileModel != "" && localModel != "" && fileModel == localModel && len(n.Embedding) > 0 {
		vec := bytesToFloat32(n.Embedding)
		if len(vec) > 0 {
			u.annotating.Add(1)
			go func() {
				defer u.annotating.Add(-1)
				u.repo.SetNodeEmbeddingByHash(context.Background(), projectID, n.File, n.Symbol, n.Kind, n.ContentHash, vec)
			}()
			return
		}
	}
	u.annotating.Add(1)
	go func(filePath, symbol, kind, hash, desc string) {
		defer u.annotating.Add(-1)
		vec, err := u.embed.Embed(context.Background(), desc)
		if err != nil {
			return
		}
		u.repo.SetNodeEmbeddingByHash(context.Background(), projectID, filePath, symbol, kind, hash, vec)
	}(n.File, n.Symbol, n.Kind, n.ContentHash, n.Description)
}

func langFromExt(filePath string) string {
	switch {
	case strings.HasSuffix(filePath, ".go"):
		return "go"
	case strings.HasSuffix(filePath, ".ts"), strings.HasSuffix(filePath, ".tsx"):
		return "typescript"
	case strings.HasSuffix(filePath, ".js"), strings.HasSuffix(filePath, ".jsx"):
		return "javascript"
	case strings.HasSuffix(filePath, ".py"):
		return "python"
	case strings.HasSuffix(filePath, ".rb"):
		return "ruby"
	case strings.HasSuffix(filePath, ".java"):
		return "java"
	case strings.HasSuffix(filePath, ".rs"):
		return "rust"
	default:
		return "unknown"
	}
}

func bytesToFloat32(b []byte) []float32 {
	if len(b)%4 != 0 {
		return nil
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}
