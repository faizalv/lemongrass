package recon_test

import (
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/modules/recon/internal/usecase"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang/golang"
)

func TestSmoke(t *testing.T) {
	uc := usecase.New(golang.New())
	tree, err := uc.Build("../../")
	if err != nil {
		t.Fatal(err)
	}
	if tree.Module != "github.com/faizalv/lemongrass" {
		t.Fatalf("unexpected module: %s", tree.Module)
	}
	if len(tree.Packages) == 0 {
		t.Fatal("no packages found")
	}
	out := uc.Format(tree)
	if !strings.Contains(out, "modules/recon") {
		t.Error("expected recon package in output")
	}
	t.Logf("packages: %d\n\n%s", len(tree.Packages), out)
}
