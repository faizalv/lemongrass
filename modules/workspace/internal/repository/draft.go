package repository

import (
	"context"
	"sync"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
)

type DraftRepository struct {
	mu     sync.Mutex
	drafts map[string]map[string]entity.TaskDecision
}

func NewDraft() *DraftRepository {
	return &DraftRepository{
		drafts: make(map[string]map[string]entity.TaskDecision),
	}
}

func (r *DraftRepository) SaveDecision(_ context.Context, workspaceID, taskID string, approved bool, feedback string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.drafts[workspaceID] == nil {
		r.drafts[workspaceID] = make(map[string]entity.TaskDecision)
	}
	r.drafts[workspaceID][taskID] = entity.TaskDecision{Approved: approved, Feedback: feedback}
	return nil
}

func (r *DraftRepository) GetDraft(_ context.Context, workspaceID string) (map[string]entity.TaskDecision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	src := r.drafts[workspaceID]
	out := make(map[string]entity.TaskDecision, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out, nil
}

func (r *DraftRepository) ClearDraft(_ context.Context, workspaceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.drafts, workspaceID)
	return nil
}
