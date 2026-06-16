package usecase

import (
	"context"
	"path/filepath"
	"time"
)

func (u *ReconUsecase) Activate(projectID int64) {
	u.syncMu.Lock()
	u.activeID = projectID
	if u.syncing[projectID] {
		u.syncMu.Unlock()
		return
	}
	u.syncing[projectID] = true
	u.syncMu.Unlock()

	go func() {
		defer func() {
			u.syncMu.Lock()
			u.syncing[projectID] = false
			u.lastSynced[projectID] = time.Now().UnixNano()
			u.syncMu.Unlock()
		}()
		rawPath, err := u.repo.ProjectDir(context.Background(), projectID)
		if err != nil {
			return
		}
		dir := "/projects/" + filepath.Base(rawPath)
		u.Sync(context.Background(), projectID, dir)
	}()
}

func (u *ReconUsecase) SyncStatus(projectID int64) (syncing bool, lastSyncedNano int64) {
	u.syncMu.Lock()
	defer u.syncMu.Unlock()
	return u.syncing[projectID], u.lastSynced[projectID]
}

func (u *ReconUsecase) TickScheduler(ctx context.Context) {
	u.syncMu.Lock()
	activeID := u.activeID
	u.syncMu.Unlock()
	if activeID == 0 {
		return
	}
	interval, err := u.repo.GetSyncInterval(ctx, activeID)
	if err != nil || interval == "off" {
		return
	}
	dur := intervalDuration(interval)
	if dur == 0 {
		return
	}
	u.syncMu.Lock()
	lastNano := u.lastSynced[activeID]
	u.syncMu.Unlock()
	if lastNano > 0 && time.Since(time.Unix(0, lastNano)) < dur {
		return
	}
	u.Activate(activeID)
}

func (u *ReconUsecase) UpdateSyncInterval(ctx context.Context, projectID int64, interval string) error {
	return u.repo.UpdateSyncInterval(ctx, projectID, interval)
}

func (u *ReconUsecase) GetSyncInterval(ctx context.Context, projectID int64) (string, error) {
	return u.repo.GetSyncInterval(ctx, projectID)
}

func (u *ReconUsecase) DeleteExpiredOrphans(ctx context.Context) {
	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	_ = u.repo.DeleteExpiredOrphans(ctx, cutoff)
}

func (u *ReconUsecase) Prune(ctx context.Context, projectID int64, orphanDays int) (superseded, orphans int, err error) {
	superseded, err = u.repo.PruneSuperseded(ctx, projectID)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-time.Duration(orphanDays) * 24 * time.Hour)
	orphans, err = u.repo.PruneOrphans(ctx, projectID, cutoff)
	return
}

func intervalDuration(s string) time.Duration {
	switch s {
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	}
	return 0
}
