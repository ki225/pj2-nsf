package model_test

import (
	"nsf/model"
	"os"
	"path/filepath"
	"testing"

	"github.com/free-ran-ue/util"
	"github.com/google/go-cmp/cmp"
)

const (
	CONFIG_FILE = "../config.yaml"
)

var targetNsfConfig = model.NsfConfig{
	Nsf: model.NsfIE{
		SbiAddress: "0.0.0.0",
		SbiPort:    8000,
		DbType:     "bbolt",
	},
	Logger: model.LoggerIE{
		Level: "info",
	},
}

func TestNsfConfigWithoutMongoDB(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("nsf:\n  sbi_address: 0.0.0.0\n  sbi_port: 8000\n  db_type: bbolt\nlogger:\n  level: info\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var config model.NsfConfig
	if err := util.LoadFromYaml(configPath, &config); err != nil {
		t.Fatalf("legacy config should remain valid: %v", err)
	}
	want := targetNsfConfig
	want.Nsf.MongoDB = model.MongoDBIE{}
	if diff := cmp.Diff(want, config); diff != "" {
		t.Fatalf("legacy config mismatch (-want +got):\n%s", diff)
	}
}

func TestNsfConfig(t *testing.T) {
	nsfConfig := model.NsfConfig{}
	if err := util.LoadFromYaml(CONFIG_FILE, &nsfConfig); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	if diff := cmp.Diff(targetNsfConfig, nsfConfig); diff != "" {
		t.Errorf("NsfConfig mismatch (-want +got):\n%s", diff)
	}
}

func TestNsfConfigWithMongoDB(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("nsf:\n  sbi_address: 0.0.0.0\n  sbi_port: 8000\n  db_type: custom\n  mongodb:\n    uri: mongodb://127.0.0.1:27017\n    name: nsf\nlogger:\n  level: info\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var config model.NsfConfig
	if err := util.LoadFromYaml(configPath, &config); err != nil {
		t.Fatal(err)
	}
	want := targetNsfConfig
	want.Nsf.DbType = "custom"
	want.Nsf.MongoDB = model.MongoDBIE{URI: "mongodb://127.0.0.1:27017", Name: "nsf"}
	if diff := cmp.Diff(want, config); diff != "" {
		t.Fatalf("custom config mismatch (-want +got):\n%s", diff)
	}
}
