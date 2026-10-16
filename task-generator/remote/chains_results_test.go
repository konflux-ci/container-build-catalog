package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tektonapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"sigs.k8s.io/yaml"
)

func TestBaseImageChainsResults(t *testing.T) {
	for _, variant := range []string{"buildah", "buildah-oci-ta", "buildah-oci-ta-min", "buildah-remote", "buildah-remote-oci-ta"} {
		t.Run(variant, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../../task", variant, variant+".yaml"))
			if err != nil {
				t.Fatal(err)
			}
			var task tektonapi.Task
			if err := yaml.Unmarshal(data, &task); err != nil {
				t.Fatal(err)
			}
			var script string
			for _, step := range task.Spec.Steps {
				if step.Name == "build" {
					script = step.Script
				}
			}
			var names []string
			for _, res := range task.Spec.Results {
				if strings.HasSuffix(res.Name, "ARTIFACT_INPUTS") {
					if res.Type != "object" || len(res.Properties) != 2 || res.Properties["uri"].Type != "string" || res.Properties["digest"].Type != "string" {
						t.Fatalf("invalid Chains result declaration: %s", res.Name)
					}
					names = append(names, res.Name)
					flag := `"$(results.` + res.Name + `.path)"`
					if strings.Count(script, flag) != 1 {
						t.Fatalf("missing or repeated result slot: %s", res.Name)
					}
				}
			}
			if len(names) != 5 {
				t.Fatal("expected five task-managed artifact outputs")
			}
			if !strings.Contains(strings.ReplaceAll(script, "> ", ">"), `>"$(results.BASE_IMAGES_DIGESTS.path)"`) ||
				!strings.Contains(script, "--resolved-base-images-output /shared/base_images_digests") ||
				!strings.Contains(script, "--resolved-base-images-json-output /shared/base_image_artifacts.json") {
				t.Fatal("missing base image output wiring")
			}
			if variant == "buildah" {
				testBaseImageAdapter(t, script)
			}
		})
	}
}

// Execute the source task adapter, rather than duplicating its jq.
func testBaseImageAdapter(t *testing.T, script string) {
	t.Helper()
	start := strings.Index(script, "base_image_inputs=")
	if start < 0 {
		t.Fatal("missing artifacts adapter")
	}
	end := strings.Index(script[start:], "jq -j .image_url")
	if end < 0 {
		t.Fatal("missing end of artifacts adapter")
	}
	block := script[start : start+end]
	dir := t.TempDir()
	input := filepath.Join(dir, "artifacts.json")
	legacy := filepath.Join(dir, "legacy")
	block = strings.ReplaceAll(block, "/shared/base_image_artifacts.json", input)
	block = strings.ReplaceAll(block, "$(results.BASE_IMAGES_DIGESTS.path)", legacy)
	var slots []string
	for i := range 5 {
		path := filepath.Join(dir, fmt.Sprintf("slot%d", i))
		slots = append(slots, path)
		block = strings.ReplaceAll(block, fmt.Sprintf("$(results.BASE_IMAGE_%d_ARTIFACT_INPUTS.path)", i+1), path)
	}
	execute := func() error {
		out, err := exec.CommandContext(t.Context(), "bash", "-euo", "pipefail", "-c", block).CombinedOutput()
		if err != nil {
			return fmt.Errorf("adapter: %w: %s", err, out)
		}
		return nil
	}
	run := func(count int, duplicate bool, invalid bool) error {
		t.Helper()
		values := make([]tektonapi.ArtifactValue, 0, count)
		for i := range count {
			values = append(values, tektonapi.ArtifactValue{Uri: fmt.Sprintf("oci://registry.example.com/base%d:Release", i), Digest: map[tektonapi.Algorithm]string{"sha256": fmt.Sprintf("%064x", i+1)}})
		}
		if duplicate {
			values = append(values, values[0])
		}
		if invalid {
			values[0].Digest["sha256"] = "bad"
		}
		data, err := json.Marshal(tektonapi.Artifacts{Inputs: []tektonapi.Artifact{{Name: "base-images", Values: values}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(input, data, 0600); err != nil {
			t.Fatal(err)
		}
		return execute()
	}
	if err := run(5, true, false); err != nil {
		t.Fatal(err)
	}
	for i, path := range slots {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]string
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got["uri"] != fmt.Sprintf("oci://registry.example.com/base%d:Release", i) || got["digest"] != fmt.Sprintf("sha256:%064x", i+1) {
			t.Fatalf("invalid Chains input: %s", data)
		}
	}
	before, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatal(err)
	}
	var wantLegacy strings.Builder
	for _, i := range []int{0, 1, 2, 3, 4, 0} {
		fmt.Fprintf(&wantLegacy, "registry.example.com/base%d:Release@sha256:%064x\n", i, i+1)
	}
	if string(before) != wantLegacy.String() {
		t.Fatalf("legacy references or repeated inputs changed: %s", before)
	}
	for _, tc := range []struct {
		count   int
		invalid bool
	}{{6, false}, {1, true}} {
		if err := run(tc.count, false, tc.invalid); err == nil {
			t.Fatal("expected overflow or invalid digest rejection")
		}
		after, err := os.ReadFile(legacy)
		if err != nil || string(after) != string(before) {
			t.Fatal("failed validation changed legacy result")
		}
	}

	for _, malformed := range []string{
		"", "not-json", `{}`, `{"inputs":[]}`, `{"inputs":{}}`,
		`{"inputs":[{"name":"wrong-group","values":[]}]}`,
		`{"inputs":[{"name":"base-images","values":[]},{"name":"base-images","values":[]}]}`,
		`{"inputs":[{"name":"base-images","values":false}]}`,
		`{"inputs":[{"name":"base-images","values":[]}]}` + "\n" + `{}`,
	} {
		if err := os.WriteFile(input, []byte(malformed), 0600); err != nil {
			t.Fatal(err)
		}
		if err := execute(); err == nil {
			t.Fatalf("accepted malformed artifacts: %q", malformed)
		}
		after, err := os.ReadFile(legacy)
		if err != nil || string(after) != string(before) {
			t.Fatal("malformed artifacts changed legacy result")
		}
	}
	if err := run(1, false, false); err != nil {
		t.Fatal(err)
	}
	for _, path := range slots[1:] {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("stale input: %s", path)
		}
	}
	if err := run(0, false, false); err != nil {
		t.Fatal(err)
	}
	for _, path := range slots {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("scratch retained an input")
		}
	}
	data, err := os.ReadFile(legacy)
	if err != nil || len(data) != 0 {
		t.Fatal("scratch must emit an empty legacy result")
	}
}
