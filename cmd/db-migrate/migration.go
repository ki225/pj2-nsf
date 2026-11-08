package main

import (
	"fmt"
	"nsf/internal/db"
	"nsf/model"
	"os"
	"time"

	"go.etcd.io/bbolt"
)

type MigrationResult struct {
	SourceCount   int
	TargetCount   int
	MigratedCount int
}

// MigrateBbolt copies services explicitly; normal NSF startup never migrates.
// Stop writers to both databases first. The source file is opened read-only.
func MigrateBbolt(sourcePath, uri, dbName string, dryRun bool) (MigrationResult, error) {
	result := MigrationResult{}
	if _, err := os.Stat(sourcePath); err != nil {
		return result, fmt.Errorf("source file: %w", err)
	}
	source, err := bbolt.Open(sourcePath, 0600, &bbolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		return result, fmt.Errorf("open source (stop the bbolt NSF first): %w", err)
	}
	defer func() { _ = source.Close() }()
	data, err := readBboltServices(source)
	if err != nil {
		return result, err
	}
	result.SourceCount = len(data)
	target, err := db.NewDb("custom", model.MongoDBIE{URI: uri, Name: dbName})
	if err != nil {
		return result, err
	}
	defer func() { _ = target.Release() }()
	existing, err := target.LoadAll()
	if err != nil {
		return result, err
	}
	result.TargetCount = len(existing)
	if len(existing) != 0 {
		return result, fmt.Errorf("target services collection is not empty; use a new database to avoid overwriting data")
	}
	if dryRun {
		return result, nil
	}
	for key, value := range data {
		if err := target.Save(key, value); err != nil {
			return result, fmt.Errorf("import %s: %w; partial target data may remain, source is unchanged", key, err)
		}
		result.MigratedCount++
	}
	saved, err := target.LoadAll()
	if err != nil {
		return result, fmt.Errorf("verify target: %w", err)
	}
	if len(saved) != len(data) {
		return result, fmt.Errorf("verification failed: source=%d, target=%d", len(data), len(saved))
	}
	for key, value := range data {
		if got, exists := saved[key]; !exists || got != value {
			return result, fmt.Errorf("verification failed for service %s", key)
		}
	}
	return result, nil
}

func readBboltServices(source *bbolt.DB) (map[string]string, error) {
	data := make(map[string]string)
	err := source.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte("services"))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(key, value []byte) error {
			data[string(key)] = string(value)
			return nil
		})
	})
	return data, err
}
