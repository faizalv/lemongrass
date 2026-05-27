package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/faizalv/lemongrass/modules/workspace/entity"
	"github.com/redis/go-redis/v9"
)

const draftTTL = 48 * time.Hour

type DraftRepository struct {
	rds *redis.Client
}

func NewDraft(rds *redis.Client) *DraftRepository {
	return &DraftRepository{rds: rds}
}

func draftKey(workspaceID string) string {
	return "lg:checkpoint:" + workspaceID
}

func (r *DraftRepository) SaveDecision(ctx context.Context, workspaceID, taskID string, approved bool, feedback string) error {
	val, _ := json.Marshal(entity.TaskDecision{Approved: approved, Feedback: feedback})
	key := draftKey(workspaceID)
	if err := r.rds.HSet(ctx, key, taskID, val).Err(); err != nil {
		return err
	}
	return r.rds.Expire(ctx, key, draftTTL).Err()
}

func (r *DraftRepository) GetDraft(ctx context.Context, workspaceID string) (map[string]entity.TaskDecision, error) {
	vals, err := r.rds.HGetAll(ctx, draftKey(workspaceID)).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string]entity.TaskDecision, len(vals))
	for taskID, raw := range vals {
		var d entity.TaskDecision
		if json.Unmarshal([]byte(raw), &d) == nil {
			out[taskID] = d
		}
	}
	return out, nil
}

func (r *DraftRepository) ClearDraft(ctx context.Context, workspaceID string) error {
	return r.rds.Del(ctx, draftKey(workspaceID)).Err()
}
