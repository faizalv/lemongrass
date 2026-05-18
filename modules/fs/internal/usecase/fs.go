package usecase

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/faizalv/lemongrass/bus"
	"github.com/faizalv/lemongrass/modules/fs/entity"
	"github.com/faizalv/lemongrass/modules/fs/internal/repository"
)

const browseCacheTTL = 5 * time.Minute

type FsUsecase struct {
	repo     *repository.FsRepository
	sockPath string

	cacheMu   sync.Mutex
	cacheTree []entity.Node
	cacheAt   time.Time
}

func New(repo *repository.FsRepository, sockPath string) *FsUsecase {
	return &FsUsecase{repo: repo, sockPath: sockPath}
}

func (uc *FsUsecase) Browse(force bool) ([]entity.Node, error) {
	if !force {
		uc.cacheMu.Lock()
		if len(uc.cacheTree) > 0 && time.Since(uc.cacheAt) < browseCacheTTL {
			nodes := uc.cacheTree
			uc.cacheMu.Unlock()
			return nodes, nil
		}
		uc.cacheMu.Unlock()
	}

	nodes, err := uc.browseFromDaemon()
	if err != nil {
		return nil, err
	}

	uc.cacheMu.Lock()
	uc.cacheTree = nodes
	uc.cacheAt = time.Now()
	uc.cacheMu.Unlock()

	return nodes, nil
}

func (uc *FsUsecase) browseFromDaemon() ([]entity.Node, error) {
	conn, err := net.DialTimeout("unix", uc.sockPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("fs daemon not reachable: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	fmt.Fprintln(conn, "BROWSE")

	var paths []string
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			paths = append(paths, line)
		}
	}

	return buildTree(paths), nil
}

func (uc *FsUsecase) Attach(path string) error {
	if _, err := uc.repo.Save(path); err != nil {
		return err
	}

	projects, err := uc.repo.ListNonRemoved()
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("unix", uc.sockPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("fs daemon not reachable: %w", err)
	}
	defer conn.Close()

	w := bufio.NewWriter(conn)
	fmt.Fprintln(w, "REMOUNT")
	for _, p := range projects {
		fmt.Fprintln(w, p.Path)
	}
	return w.Flush()
}

func (uc *FsUsecase) RemoveProject(id int64) error {
	if err := uc.repo.UpdateStatus(id, "removed"); err != nil {
		return err
	}

	bus.Default.Emit(bus.EventProjectRemoved, id)

	projects, err := uc.repo.ListNonRemoved()
	if err != nil {
		return err
	}

	conn, err := net.DialTimeout("unix", uc.sockPath, 5*time.Second)
	if err != nil {
		return nil // daemon may not be running; removal is recorded regardless
	}
	defer conn.Close()

	w := bufio.NewWriter(conn)
	fmt.Fprintln(w, "REMOUNT")
	for _, p := range projects {
		fmt.Fprintln(w, p.Path)
	}
	return w.Flush()
}

func (uc *FsUsecase) ListProjects() ([]entity.Project, error) {
	return uc.repo.List()
}

func (uc *FsUsecase) RunSanityCheck(ctx context.Context) {
	projects, err := uc.repo.ListNonRemoved()
	if err != nil {
		return
	}
	for _, p := range projects {
		status := "active"
		if _, err := os.Stat(p.Path); err != nil {
			status = "missing"
		}
		uc.repo.UpdateStatus(p.ID, status)
	}
}

type treeNode struct {
	name     string
	path     string
	children []*treeNode
}

func buildTree(paths []string) []entity.Node {
	nodeMap := make(map[string]*treeNode, len(paths))
	var roots []*treeNode

	for _, p := range paths {
		n := &treeNode{name: filepath.Base(p), path: p}
		nodeMap[p] = n

		parent := filepath.Dir(p)
		if pn, ok := nodeMap[parent]; ok {
			pn.children = append(pn.children, n)
		} else {
			roots = append(roots, n)
		}
	}

	result := make([]entity.Node, len(roots))
	for i, r := range roots {
		result[i] = toEntityNode(r)
	}
	return result
}

func toEntityNode(n *treeNode) entity.Node {
	children := make([]entity.Node, len(n.children))
	for i, c := range n.children {
		children[i] = toEntityNode(c)
	}
	return entity.Node{Name: n.name, Path: n.path, Children: children}
}
