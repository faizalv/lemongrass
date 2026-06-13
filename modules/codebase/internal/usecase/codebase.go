package usecase

import (
	"context"
	"fmt"

	"github.com/faizalv/lemongrass/bus"
)

type CodebaseUsecase struct {
	cache *lruCache
	sf    *singleflightGroup
}

func New() *CodebaseUsecase {
	uc := &CodebaseUsecase{
		cache: newLRUCache(10 * 1024 * 1024),
		sf:    &singleflightGroup{},
	}
	bus.Default.On(bus.EventFileChanged, func(payload any) {
		if id, ok := payload.(int64); ok {
			uc.cache.invalidateProject(id)
		}
	})
	return uc
}

func (u *CodebaseUsecase) Ls(_ context.Context, projectID int64, projectDir, args string) string {
	key := fmt.Sprintf("%d:ls:%s", projectID, args)
	return u.cached(key, projectID, func() string {
		return ls(projectDir, args)
	})
}

func (u *CodebaseUsecase) Files(_ context.Context, projectID int64, projectDir, args string) string {
	key := fmt.Sprintf("%d:files:%s", projectID, args)
	return u.cached(key, projectID, func() string {
		return files(projectDir, args)
	})
}

func (u *CodebaseUsecase) Search(_ context.Context, projectID int64, projectDir string, filePaths []string, args string) string {
	key := fmt.Sprintf("%d:search:%s", projectID, args)
	return u.cached(key, projectID, func() string {
		return search(projectDir, filePaths, args)
	})
}

func (u *CodebaseUsecase) cached(key string, projectID int64, fn func() string) string {
	if v, ok := u.cache.get(key); ok {
		return v
	}
	result := u.sf.Do(key, fn)
	u.cache.set(key, projectID, result)
	return result
}
