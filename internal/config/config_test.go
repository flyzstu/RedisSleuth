package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMergesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("redis:\n  addr: 10.0.0.1:6379\nsampling:\n  duration: 7s\noutput:\n  format: json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Redis.Addr != "10.0.0.1:6379" || got.Redis.CommandTimeout != 2*time.Second {
		t.Fatalf("Redis 默认值未保留: %#v", got.Redis)
	}
	if got.Sampling.Duration != 7*time.Second || got.Sampling.ScanRate != 500 {
		t.Fatalf("Sampling 默认值未保留: %#v", got.Sampling)
	}
	if got.Output.Format != "json" {
		t.Fatalf("Output=%#v", got.Output)
	}
}
