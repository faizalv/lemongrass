package recon_test

import (
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/modules/recon/internal/usecase"
	"github.com/faizalv/lemongrass/modules/recon/internal/usecase/lang/golang"
)

func TestSmoke(t *testing.T) {
	uc := usecase.New(nil, golang.New())
	trees, err := uc.Build("../../")
	if err != nil {
		t.Fatal(err)
	}
	if len(trees) == 0 {
		t.Fatal("no trees returned")
	}
	goTree := trees[0]
	if goTree.Module != "github.com/faizalv/lemongrass" {
		t.Fatalf("unexpected module: %s", goTree.Module)
	}
	if len(goTree.Packages) == 0 {
		t.Fatal("no packages found")
	}
	out := uc.Format(goTree)
	if !strings.Contains(out, "modules/recon") {
		t.Error("expected recon package in output")
	}
	t.Logf("packages: %d\n\n%s", len(goTree.Packages), out)
}
