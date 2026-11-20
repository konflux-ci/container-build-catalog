package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"sigs.k8s.io/yaml"
)

const taskVersionLabel = "app.kubernetes.io/version"

func readTaskVersion(taskPath string) (string, error) {
	content, err := os.ReadFile(taskPath)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", taskPath, err)
	}

	var task struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
	}
	if err := yaml.Unmarshal(content, &task); err != nil {
		return "", fmt.Errorf("failed to unmarshal %s: %w", taskPath, err)
	}

	version, ok := task.Metadata.Labels[taskVersionLabel]
	if !ok {
		return "", fmt.Errorf("failed to get %s label from %s", taskVersionLabel, taskPath)
	}
	return version, nil
}

func collectMigrationScripts(taskNames []string) ([]string, error) {
	return collectMigrationScriptsFrom(filepath.Join("..", "task"), taskNames)
}

func collectMigrationScriptsFrom(taskRoot string, taskNames []string) ([]string, error) {
	var scripts []string
	for _, taskName := range taskNames {
		taskDir := filepath.Join(taskRoot, taskName)
		taskFile := filepath.Join(taskDir, taskName+".yaml")

		version, err := readTaskVersion(taskFile)
		if err != nil {
			return nil, err
		}

		// A task version does not have to come with a migration.
		script := filepath.Join(taskDir, "migrations", version+".sh")
		if _, err := os.Stat(script); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("failed to look up migration file %s: %w", script, err)
		}
		scripts = append(scripts, script)
	}
	return scripts, nil
}

func applyTaskMigrations(pipelineYaml []byte, taskNames []string) ([]byte, error) {
	scripts, err := collectMigrationScripts(taskNames)
	if err != nil {
		return nil, err
	}

	pipelineFile, err := os.CreateTemp("", "pipeline-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary pipeline file: %w", err)
	}
	defer os.Remove(pipelineFile.Name())

	if _, err := pipelineFile.Write(pipelineYaml); err != nil {
		return nil, fmt.Errorf("failed to write temporary pipeline file: %w", err)
	}

	for _, script := range scripts {
		cmd := exec.Command("bash", script, pipelineFile.Name())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to run migration script %s: %w", script, err)
		}
	}

	migrated, err := os.ReadFile(pipelineFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read pipeline file: %w", err)
	}
	return migrated, nil
}
