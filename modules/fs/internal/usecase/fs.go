package usecase

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/faizalv/lemongrass/modules/fs/entity"
	"github.com/faizalv/lemongrass/modules/fs/internal/repository"
)

type FsUsecase struct {
	repo     *repository.FsRepository
	sockPath string
}

func New(repo *repository.FsRepository, sockPath string) *FsUsecase {
	return &FsUsecase{repo: repo, sockPath: sockPath}
}

func (uc *FsUsecase) Browse() ([]entity.Node, error) {
	conn, err := net.DialTimeout("unix", uc.sockPath, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("fs daemon not reachable: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(30 * time.Second))

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
