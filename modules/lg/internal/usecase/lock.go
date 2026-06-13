package usecase

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func normalizePath(path string) string {
	if abs, err := filepath.Abs(filepath.Clean(path)); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

func (u *LgUsecase) AcquireLock(sessionID, path string) (holderID string, err error) {
	path = normalizePath(path)
	u.mu.Lock()
	defer u.mu.Unlock()

	s := u.sessions[sessionID]
	if s == nil || s.sessionType != "execution" {
		return "", nil
	}
	if _, held := s.locks[path]; held {
		return "", nil
	}
	for id, sess := range u.sessions {
		if id == sessionID {
			continue
		}
		if _, held := sess.locks[path]; held {
			return id, fmt.Errorf("file locked by another session")
		}
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return "", err
	}
	if ferr := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); ferr != nil {
		f.Close()
		if ferr == syscall.EWOULDBLOCK {
			return "external process", fmt.Errorf("file locked by external process")
		}
		return "", ferr
	}

	if s.locks == nil {
		s.locks = make(map[string]*os.File)
	}
	s.locks[path] = f
	return "", nil
}

func (u *LgUsecase) ReleaseLock(sessionID, path string) {
	path = normalizePath(path)
	u.mu.Lock()
	defer u.mu.Unlock()
	s := u.sessions[sessionID]
	if s != nil {
		releaseOneLock(s.locks, path)
	}
}

func releaseOneLock(locks map[string]*os.File, path string) {
	f, ok := locks[path]
	if !ok {
		return
	}
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()
	delete(locks, path)
}

func releaseSessionLocks(s *activeSession) {
	for path, f := range s.locks {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
		delete(s.locks, path)
	}
}
