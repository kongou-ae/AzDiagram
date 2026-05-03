package generator_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/kongou-ae/AzDiagram/internal/generator"
)

// Run with -update to regenerate the golden file:
//
//	go test ./internal/generator/ -run TestGolden_Webapp -update
var update = flag.Bool("update", false, "update golden files")

func TestGolden_Webapp(t *testing.T) {
	opts := generator.Options{
		InputFile: filepath.Join("testdata", "webapp.bicep"),
		// IconsDir intentionally omitted: icon files are git-ignored.
	}

	got, err := generator.GenerateSVG(opts)
	if err != nil {
		t.Fatalf("GenerateSVG: %v", err)
	}

	goldenPath := filepath.Join("testdata", "webapp.golden")

	if *update {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("updated %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v\n\nTo create it, run:\n  go test ./internal/generator/ -run TestGolden_Webapp -update", goldenPath, err)
	}

	if got != string(want) {
		t.Errorf("SVG output does not match golden file.\n\nTo update it, run:\n  go test ./internal/generator/ -run TestGolden_Webapp -update")
	}
}
