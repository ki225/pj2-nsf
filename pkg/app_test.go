package pkg_test

import (
	"fmt"
	"nsf/internal/db"
	"nsf/model"
	"nsf/pkg"
	"os"
	"path/filepath"
	"testing"
)

func TestMongoDBConfig(t *testing.T) {
	if os.Getenv("NSF_TEST_CUSTOM_DB") != "1" {
		t.Skip("set NSF_TEST_CUSTOM_DB=1 to test against a dedicated MongoDB database")
	}
	uri := os.Getenv("NSF_MONGODB_URI")
	if uri == "" {
		t.Fatal("set NSF_MONGODB_URI to the test MongoDB address")
	}
	dbName := "nsf_config_validation"
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	config := fmt.Sprintf("nsf:\n  sbi_address: 127.0.0.1\n  sbi_port: 8000\n  db_type: custom\n  mongodb:\n    uri: %q\n    name: %q\nlogger:\n  level: error\n", uri, dbName)
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	app, err := pkg.NewApp(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if app.NsfContext == nil {
		t.Fatal("failed to initialize context from YAML")
	}
	defer app.NsfContext.Release()
	if err := app.RegisterServices("NF1", "127.0.0.1", 8080, []string{"config-test-service"}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := app.DeregisterServices("NF1", []string{"config-test-service"}); err != nil {
			t.Error(err)
		}
	}()
	store, err := db.NewDb("custom", model.MongoDBIE{URI: uri, Name: dbName})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.Release(); err != nil {
			t.Error(err)
		}
	}()
	exists, err := store.Exist("config-test-service")
	if err != nil || !exists {
		t.Fatalf("service was not saved to the database selected in YAML: exists=%v, err=%v", exists, err)
	}
}
