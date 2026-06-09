package usecase

import (
	"context"
	"strings"

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
