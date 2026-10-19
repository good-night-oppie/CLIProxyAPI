package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizedDeploymentExamplesLoad(t *testing.T) {
	for _, name := range []string{"europa-oauth.yaml", "europa-gemini.yaml", "win11-oauth.yaml", "win11-gemini.yaml"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "deploy", "examples", name))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Port < 1 {
				t.Fatal("example must specify its service port")
			}
		})
	}
}
