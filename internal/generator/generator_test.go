package generator_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/kongou-ae/AzDiagram/internal/generator"
)

// Run with -update to regenerate golden files:
//
//	go test ./internal/generator/ -update
var update = flag.Bool("update", false, "update golden files")

func goldenTest(t *testing.T, bicepFile, goldenFile string) {
	t.Helper()

	opts := generator.Options{
		InputFile: filepath.Join("testdata", bicepFile),
		// IconsDir intentionally omitted: icon files are git-ignored.
	}

	got, err := generator.GenerateSVG(opts)
	if err != nil {
		t.Fatalf("GenerateSVG: %v", err)
	}

	goldenPath := filepath.Join("testdata", goldenFile)

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
		t.Fatalf("read golden %s: %v\n\nTo create it, run:\n  go test ./internal/generator/ -run %s -update",
			goldenPath, err, t.Name())
	}

	if got != string(want) {
		t.Errorf("SVG output does not match golden file.\n\nTo update it, run:\n  go test ./internal/generator/ -run %s -update",
			t.Name())
	}
}

func TestGolden_Example(t *testing.T) { goldenTest(t, "example.bicep", "example.golden") }
func TestGolden_Main(t *testing.T)    { goldenTest(t, "main.bicep", "main.golden") }
