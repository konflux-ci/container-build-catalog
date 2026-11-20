package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestReadTaskVersion(t *testing.T) {
	content := `
apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: my-task
  labels:
    app.kubernetes.io/version: 0.1.0
  annotations:
    tekton.dev/tags: "konflux"
`
	tempFile, err := os.CreateTemp("", "my-task-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(tempFile.Name()) })

	if _, err := tempFile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatal(err)
	}

	version, err := readTaskVersion(tempFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	want := "0.1.0"
	if version != want {
		t.Fatalf("expected version %s, got %s", want, version)
	}
}

func TestCollectMigrationScripts(t *testing.T) {
	root := t.TempDir()
	writeTaskFixture(t, root, "task-with-migration", "0.1.0", "#!/usr/bin/env bash\nexit 0\n")
	writeTaskFixture(t, root, "task-without-migration", "0.2.0", "")

	scripts, err := collectMigrationScriptsFrom(root, []string{"task-without-migration", "task-with-migration"})
	if err != nil {
		t.Fatal(err)
	}

	if len(scripts) != 1 {
		t.Fatalf("got %d scripts, want 1: %v", len(scripts), scripts)
	}

	want := filepath.Join(root, "task-with-migration", "migrations", "0.1.0.sh")
	if scripts[0] != want {
		t.Fatalf("expected script %s, got %s", want, scripts[0])
	}
}

func writeTaskFixture(t *testing.T, root, name, version, script string) {
	t.Helper()
	taskDir := filepath.Join(root, name)
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}

	task := fmt.Sprintf(`apiVersion: tekton.dev/v1
kind: Task
metadata:
  name: %s
  labels:
    app.kubernetes.io/version: %q
  annotations:
    tekton.dev/tags: "konflux"
`, name, version)
	if err := os.WriteFile(filepath.Join(taskDir, name+".yaml"), []byte(task), 0o644); err != nil {
		t.Fatal(err)
	}

	if script == "" {
		return
	}

	migrationsDir := filepath.Join(taskDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migrationsDir, version+".sh"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
}
