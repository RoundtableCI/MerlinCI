// Package app wires MerlinCI together: it is the one place that knows which
// concrete database, provider and connectors sit behind the interfaces the
// rest of the code uses. The three programs in cmd/ call it.
package app

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/RoundtableCI/MerlinCI/internal/blob/s3blob"
	"github.com/RoundtableCI/MerlinCI/internal/workflow"
)

// Env is MerlinCI's process configuration, read from environment variables.
// The defaults match deploy/docker-compose.yml.
type Env struct {
	ConfigDir         string
	DatabaseURL       string
	TemporalAddress   string
	TemporalNamespace string
	// TaskQueue is the Temporal task queue the API sends work to and the
	// worker takes it from.
	TaskQueue string
	APIAddr   string
	APIURL    string
	APIToken  string
	S3        s3blob.Config
}

// LoadEnv reads the environment, after loading a .env file when there is one.
func LoadEnv() (Env, error) {
	if err := LoadDotEnv(".env"); err != nil {
		return Env{}, err
	}
	return Env{
		ConfigDir:         get("MERLIN_CONFIG_DIR", "configs"),
		DatabaseURL:       get("MERLIN_DATABASE_URL", "postgres://merlin:merlin@localhost:5432/merlin?sslmode=disable"),
		TemporalAddress:   get("MERLIN_TEMPORAL_ADDRESS", "localhost:7233"),
		TemporalNamespace: get("MERLIN_TEMPORAL_NAMESPACE", "default"),
		TaskQueue:         get("MERLIN_TEMPORAL_TASK_QUEUE", workflow.TaskQueue),
		APIAddr:           get("MERLIN_API_ADDR", ":8080"),
		APIURL:            get("MERLIN_API_URL", "http://localhost:8080"),
		APIToken:          os.Getenv("MERLIN_API_TOKEN"),
		S3: s3blob.Config{
			Endpoint:        get("MERLIN_S3_ENDPOINT", "http://localhost:8333"),
			Region:          get("MERLIN_S3_REGION", "us-east-1"),
			Bucket:          get("MERLIN_S3_BUCKET", "merlin-artifacts"),
			AccessKeyID:     os.Getenv("MERLIN_S3_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("MERLIN_S3_SECRET_ACCESS_KEY"),
		},
	}, nil
}

func get(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// LoadDotEnv sets environment variables from a file of NAME=value lines. A
// variable that is already set keeps its value, so the real environment
// always wins. A missing file is not an error.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		name = strings.TrimSpace(strings.TrimPrefix(name, "export "))
		if !ok || name == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if _, set := os.LookupEnv(name); !set {
			if err := os.Setenv(name, value); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}
