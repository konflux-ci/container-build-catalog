package main

import (
	"reflect"
	"testing"
)

func TestParseSnapshotImagesSelectsBundleMode(t *testing.T) {
	snapshot := `{"components":[
		{"name":"task-git-clone","containerImage":"quay.io/example/task-git-clone@sha256:task"},
		{"name":"pipeline-docker-build","containerImage":"quay.io/example/pipeline-docker-build@sha256:pipeline"}
	]}`

	tests := []struct {
		name string
		mode string
		want map[string]string
	}{
		{
			name: "task",
			mode: taskBundleMode,
			want: map[string]string{"git-clone": "quay.io/example/task-git-clone@sha256:task"},
		},
		{
			name: "pipeline",
			mode: pipelineBundleMode,
			want: map[string]string{"docker-build": "quay.io/example/pipeline-docker-build@sha256:pipeline"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSnapshotImages(snapshot, "group", "task-git-clone,pipeline-docker-build", tt.mode)
			if err != nil {
				t.Fatalf("ParseSnapshotImages() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSnapshotImages() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolvePipelineBundleRef(t *testing.T) {
	const snapshotPipelineBundle = "quay.io/example/pipeline-docker-build@sha256:updated"

	got, err := resolvePipelineBundleRef("docker-build", map[string]string{
		"docker-build": snapshotPipelineBundle,
	})
	if err != nil {
		t.Fatalf("resolvePipelineBundleRef() error = %v", err)
	}
	if got != snapshotPipelineBundle {
		t.Errorf("resolvePipelineBundleRef() = %q, want snapshot bundle %q", got, snapshotPipelineBundle)
	}

	got, err = resolvePipelineBundleRef("docker-build", nil)
	if err != nil {
		t.Fatalf("resolvePipelineBundleRef() default error = %v", err)
	}
	wantDefault, err := getDefaultPipelineBundleRef("docker-build")
	if err != nil {
		t.Fatalf("getDefaultPipelineBundleRef() error = %v", err)
	}
	if got != wantDefault {
		t.Errorf("resolvePipelineBundleRef() = %q, want default bundle %q", got, wantDefault)
	}
}
