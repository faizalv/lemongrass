package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/faizalv/lemongrass/modules/recon/entity"
)

// Sync detects file-level changes and re-maps the project if anything changed.
// On first run (no stored hashes) it always calls Map and writes the baseline.
func (u *ReconUsecase) Sync(ctx context.Context, projectID int64, dir string) error {
	ig := loadIgnore(dir)

	var parsedCandidates []string
	var ignoredExisting []string

	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		if ig.Match(rel) {
			ignoredExisting = append(ignoredExisting, rel)
		} else {
			parsedCandidates = append(parsedCandidates, rel)
		}
		return nil
	}); err != nil {
		return err
	}

	currentHashes := make(map[string]string, len(parsedCandidates))
	for _, rel := range parsedCandidates {
		h, err := hashFile(filepath.Join(dir, rel))
		if err != nil {
			continue
		}
		currentHashes[rel] = h
	}

	storedHashes, err := u.repo.GetFileHashes(ctx, projectID)
	if err != nil {
		return err
	}

	var toUpsert []entity.FileHash
	var toDelete []string
	changed := len(storedHashes) == 0 // first run always triggers Map

	for path, hash := range currentHashes {
		if stored, exists := storedHashes[path]; !exists || stored != hash {
			toUpsert = append(toUpsert, entity.FileHash{Path: path, Hash: hash})
			changed = true
		}
	}
	for path := range storedHashes {
		if _, exists := currentHashes[path]; !exists {
			toDelete = append(toDelete, path)
			changed = true
		}
	}

	if !changed {
		return nil
	}

	if err := u.Map(ctx, projectID, dir, ignoredExisting); err != nil {
		return err
	}

	if len(toUpsert) > 0 {
		if err := u.repo.UpsertFileHashes(ctx, projectID, toUpsert); err != nil {
			return err
		}
	}
	if len(toDelete) > 0 {
		if err := u.repo.DeleteFileHashes(ctx, projectID, toDelete); err != nil {
			return err
		}
	}

	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
