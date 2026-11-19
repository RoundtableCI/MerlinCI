package schemas

import (
	"strings"
	"testing"
)

const validTask = `{
  "id": "task_01",
  "role": "market-researcher",
  "status": "pending",
  "createdBy": "orchestrator",
  "input": {"topic": "model comparison"},
  "createdAt": "2026-11-13T09:00:00Z"
}`

const validArtifact = `{
  "id": "art_01",
  "kind": "market-opportunity",
  "taskId": "task_01",
  "role": "market-researcher",
  "title": "An opportunity",
  "content": {
    "mediaType": "text/markdown",
    "blobKey": "artifacts/art_01",
    "sha256": "0000000000000000000000000000000000000000000000000000000000000000",
    "sizeBytes": 120
  },
  "sources": [{"url": "https://example.com/post", "retrievedAt": "2026-11-13T09:01:00Z"}],
  "evaluations": [{"evaluator": "brand-guardian", "verdict": "pass"}],
  "cost": {"currency": "INR", "amount": "4.25", "inputTokens": 1200, "outputTokens": 300},
  "configDigest": "sha256:0000000000000000000000000000000000000000000000000000000000000000",
  "createdAt": "2026-11-13T09:02:00Z"
}`

func TestEverySchemaCompiles(t *testing.T) {
	if _, err := compiled(); err != nil {
		t.Fatal(err)
	}
}

func TestRecords(t *testing.T) {
	if err := Validate("Task", []byte(validTask)); err != nil {
		t.Errorf("valid task rejected: %v", err)
	}
	if err := Validate("Artifact", []byte(validArtifact)); err != nil {
		t.Errorf("valid artifact rejected: %v", err)
	}

	bad := map[string][2]string{
		"task with unknown status":      {"Task", strings.Replace(validTask, `"pending"`, `"paused"`, 1)},
		"task created by an agent":      {"Task", strings.Replace(validTask, `"orchestrator"`, `"agent"`, 1)},
		"task with an unknown field":    {"Task", strings.Replace(validTask, `"id":`, `"shell": 1, "id":`, 1)},
		"artifact without sources":      {"Artifact", strings.Replace(validArtifact, `"sources"`, `"origins"`, 1)},
		"artifact without cost":         {"Artifact", strings.Replace(validArtifact, `"cost"`, `"price"`, 1)},
		"artifact with numeric cost":    {"Artifact", strings.Replace(validArtifact, `"4.25"`, `4.25`, 1)},
		"artifact without config trace": {"Artifact", strings.Replace(validArtifact, `"configDigest"`, `"config"`, 1)},
	}
	for name, tc := range bad {
		if err := Validate(tc[0], []byte(tc[1])); err == nil {
			t.Errorf("%s: accepted, want rejected", name)
		}
	}
}

func TestUnknownKind(t *testing.T) {
	if Known("Agent") {
		t.Error(`Known("Agent") = true`)
	}
	if err := Validate("Agent", []byte(`{}`)); err == nil {
		t.Error("unknown kind accepted")
	}
}
