package usecase

import (
	"context"
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

func (u *ReconUsecase) ExportArtifacts(ctx context.Context, projectID int64, originID string) (*lgart.File, error) {
	nodes, err := u.repo.ListNodes(ctx, projectID, "", "", "")
	if err != nil {
		return nil, err
	}
	knowledge, err := u.repo.ListKnowledge(ctx, projectID)
	if err != nil {
		return nil, err
	}

	f := &lgart.File{
		Version:     1,
		GeneratedBy: originID,
		ExportedAt:  time.Now().UTC(),
	}

	for _, n := range nodes {
		if n.Description == "" {
			continue
		}
		f.Nodes = append(f.Nodes, lgart.Node{
			File:        n.FilePath,
			Symbol:      n.Symbol,
			Kind:        n.Kind,
			Receiver:    n.Receiver,
			Description: n.Description,
			ReturnType:  n.ReturnType,
			DependsOn:   n.Calls,
		})
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
	var result lgart.ImportResult

	for _, n := range f.Nodes {
		if !force {
			existing, err := u.repo.GetNode(ctx, projectID, n.File, n.Symbol, n.Kind)
			if err == nil && existing.Description != "" {
				result.NodesSkipped++
				continue
			}
		}
		imported, err := u.repo.AnnotateNode(ctx, projectID, n.File, n.Symbol, n.Kind, n.Description, n.ReturnType, n.DependsOn)
		if err != nil || imported == 0 {
			continue
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
