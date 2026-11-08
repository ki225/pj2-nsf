package main

import (
	"context"
	"nsf/internal/db"
	"nsf/model"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"go.etcd.io/bbolt"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestBboltToCustomMigration(t *testing.T) {
	if os.Getenv("NSF_TEST_CUSTOM_DB") != "1" {
		t.Skip("set NSF_TEST_CUSTOM_DB=1 to test against MongoDB")
	}
	uri := os.Getenv("NSF_MONGODB_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27017"
	}
	for _, scenario := range []string{"dry run", "empty target", "existing target"} {
		t.Run(scenario, func(t *testing.T) {
			dbName := "nsf_migration_test_" + uuid.NewString()
			target, err := db.NewDb("custom", model.MongoDBIE{URI: uri, Name: dbName})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				client, err := mongo.Connect(options.Client().ApplyURI(uri).SetTimeout(30 * time.Second))
				if err != nil {
					t.Error(err)
				} else {
					if err := client.Database(dbName).Drop(context.Background()); err != nil {
						t.Error(err)
					}
					if err := client.Disconnect(context.Background()); err != nil {
						t.Error(err)
					}
				}
				if err := target.Release(); err != nil {
					t.Error(err)
				}
			})
			path := filepath.Join(t.TempDir(), "source.db")
			source, err := bbolt.Open(path, 0600, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"service1": "record1", "service2": "record2"}
			if err := source.Update(func(tx *bbolt.Tx) error {
				bucket, err := tx.CreateBucketIfNotExists([]byte("services"))
				if err != nil {
					return err
				}
				for key, value := range want {
					if err := bucket.Put([]byte(key), []byte(value)); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				_ = source.Close()
				t.Fatal(err)
			}
			if err := source.Close(); err != nil {
				t.Fatal(err)
			}
			if scenario == "existing target" {
				if err := target.Save("service1", "target-owned-value"); err != nil {
					t.Fatal(err)
				}
			}
			result, migrationErr := MigrateBbolt(path, uri, dbName, scenario == "dry run")
			expected := want
			switch scenario {
			case "existing target":
				if migrationErr == nil {
					t.Fatal("expected rejection of a nonempty target")
				}
				expected = map[string]string{"service1": "target-owned-value"}
			case "dry run":
				if migrationErr != nil {
					t.Fatal(migrationErr)
				}
				if result.SourceCount != 2 || result.TargetCount != 0 || result.MigratedCount != 0 {
					t.Fatalf("unexpected preview: %+v", result)
				}
				expected = map[string]string{}
			default:
				if migrationErr != nil {
					t.Fatal(migrationErr)
				}
				if result.MigratedCount != 2 {
					t.Fatalf("unexpected result: %+v", result)
				}
				if _, err := MigrateBbolt(path, uri, dbName, false); err == nil {
					t.Fatal("repeated migration should reject the populated target")
				}
			}
			got, err := target.LoadAll()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Fatalf("target mismatch (-want +got):\n%s", diff)
			}
			source, err = bbolt.Open(path, 0600, &bbolt.Options{ReadOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = source.Close() }()
			data, err := readBboltServices(source)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(want, data); diff != "" {
				t.Fatalf("source changed (-want +got):\n%s", diff)
			}
		})
	}
}
