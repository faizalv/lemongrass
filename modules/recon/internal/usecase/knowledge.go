package usecase

import (
	"context"

	"github.com/faizalv/lemongrass/modules/recon/entity"
)

func (u *ReconUsecase) SaveKnowledge(ctx context.Context, projectID int64, key, content string) error {
	vec, _ := u.embed.Embed(ctx, content)
	return u.repo.SaveKnowledge(ctx, projectID, key, content, vec)
}

func (u *ReconUsecase) ReadKnowledge(ctx context.Context, projectID int64, key string) (string, error) {
	return u.repo.ReadKnowledge(ctx, projectID, key)
}

func (u *ReconUsecase) SearchKnowledge(ctx context.Context, projectID int64, query string) ([]entity.KnowledgeEntry, error) {
	vec, err := u.embed.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	return u.repo.SearchKnowledge(ctx, projectID, vec, 5)
}

func (u *ReconUsecase) ListKnowledge(ctx context.Context, projectID int64) ([]entity.KnowledgeEntry, error) {
	return u.repo.ListKnowledge(ctx, projectID)
}
