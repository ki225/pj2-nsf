package sbi

import (
	"context"
	"encoding/json"
	"net/http"
	"nsf/internal/db"
	"nsf/logger"
	"nsf/model"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	loggergoUtil "github.com/Alonza0314/logger-go/v2/util"
)

func TestNSFRecovery(t *testing.T) {
	f := newServiceTestServer(t, filepath.Join(t.TempDir(), "recovery.db"))
	f.register(t, "NF1", "127.0.0.1", 8080, "survivor1", "deleted")
	f.register(t, "NF2", "127.0.0.2", 8081, "survivor2")
	f.deregister(t, "NF1", "deleted")
	for restart := 0; restart < 2; restart++ {
		oldContext := f.server.NsfContext
		f.close()
		if _, _, _, err := oldContext.DiscoverService("survivor1"); err == nil {
			t.Fatal("released context still allows discovery")
		}
		if err := oldContext.RegisterServices("NF1", "127.0.0.1", 8080, []string{"after-release"}); err == nil {
			t.Fatal("released context still allows registration")
		}
		if err := oldContext.DeregisterServices("NF1", []string{"survivor1"}); err == nil {
			t.Fatal("released context still allows deregistration")
		}
		f.start(t)
		f.discover(t, model.NnsfDiscGetResponse{Service: "survivor1", NfName: "NF1", SbiAddress: "127.0.0.1", SbiPort: 8080})
		f.discover(t, model.NnsfDiscGetResponse{Service: "survivor2", NfName: "NF2", SbiAddress: "127.0.0.2", SbiPort: 8081})
		f.request(t, http.MethodGet, "/sbi/nnsf-disc", model.NnsfDiscGetRequest{Service: "deleted"}, http.StatusNotFound)
		w := f.request(t, http.MethodGet, "/sbi/nnsf-disc/group",
			model.NnsfDiscGroupGetRequest{Services: []string{"survivor2", "survivor1"}}, http.StatusOK)
		var group model.NnsfDiscGroupGetResponse
		if err := json.Unmarshal(w.Body.Bytes(), &group); err != nil {
			t.Fatal(err)
		}
		if len(group.Services) != 2 || group.Services[0].Service != "survivor2" || group.Services[1].Service != "survivor1" {
			t.Fatalf("unexpected group after recovery: %+v", group.Services)
		}
	}
	// Recovered contexts must accept new registrations and persist later deletions.
	f.register(t, "NF3", "127.0.0.3", 8082, "new-service")
	f.deregister(t, "NF1", "survivor1")
	f.close()
	f.start(t)
	f.discover(t, model.NnsfDiscGetResponse{Service: "new-service", NfName: "NF3", SbiAddress: "127.0.0.3", SbiPort: 8082})
	f.request(t, http.MethodGet, "/sbi/nnsf-disc", model.NnsfDiscGetRequest{Service: "survivor1"}, http.StatusNotFound)
}

func TestNSFRecoveryAfterProcessExit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "process-exit.db")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestNSFProcessExitWriter$")
	cmd.Env = append(os.Environ(), "NSF_RECOVERY_WRITER=1", "NSF_RECOVERY_DB="+dbPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("recovery writer failed: %v\n%s", err, output)
	}
	f := newServiceTestServer(t, dbPath)
	f.discover(t, model.NnsfDiscGetResponse{Service: "survivor", NfName: "NF1", SbiAddress: "127.0.0.1", SbiPort: 8080})
	f.request(t, http.MethodGet, "/sbi/nnsf-disc", model.NnsfDiscGetRequest{Service: "deleted"}, http.StatusNotFound)
}

// Only the subprocess runs this helper. os.Exit intentionally bypasses cleanup
// and Release(), checking committed writes survive abrupt process termination.
// This does not simulate power loss or an interrupted transaction.
func TestNSFProcessExitWriter(t *testing.T) {
	if os.Getenv("NSF_RECOVERY_WRITER") != "1" {
		t.Skip("subprocess helper")
	}
	dbPath := os.Getenv("NSF_RECOVERY_DB")
	if dbPath == "" {
		t.Fatal("missing subprocess database path")
	}
	f := newServiceTestServer(t, dbPath)
	f.register(t, "NF1", "127.0.0.1", 8080, "survivor", "deleted")
	f.deregister(t, "NF1", "deleted")
	os.Exit(0)
}

func TestNSFRecoveryRejectsCorruptRecord(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "corrupt.db")
	f := newServiceTestServer(t, dbPath)
	f.close()
	store, err := db.NewDb("bbolt")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save("broken", "invalid JSON"); err != nil {
		_ = store.Release()
		t.Fatal(err)
	}
	if err := store.Release(); err != nil {
		t.Fatal(err)
	}
	// Construction must reject unreadable persisted state rather than silently
	// starting with an empty table. It must also release the acquired DB lock.
	nsfLogger := logger.NewNsfLogger(loggergoUtil.LogLevelString("error"))
	server := NewServer(&ServerIE{DbType: "bbolt", NsfLogger: &nsfLogger})
	if server.NsfContext != nil {
		server.Processor.Release()
		t.Fatal("corrupt persisted record should prevent context initialization")
	}
	store, err = db.NewDb("bbolt")
	if err != nil {
		t.Fatalf("failed recovery left database locked: %v", err)
	}
	if err := store.Release(); err != nil {
		t.Fatal(err)
	}
}
