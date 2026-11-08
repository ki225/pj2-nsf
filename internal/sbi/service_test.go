package sbi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"nsf/internal/db"
	"nsf/logger"
	"nsf/model"
	"path/filepath"
	"sync"
	"testing"

	loggergoUtil "github.com/Alonza0314/logger-go/v2/util"
	"github.com/gin-gonic/gin"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
)

// Like the free5GC AMF router tests, exercise HTTP handlers with httptest.
// Keep setup serial: NSF_DB_PATH and Gin mode are process-global settings.
type serviceTestServer struct {
	server *Server
}

func newServiceTestServer(t *testing.T, dbPath string) *serviceTestServer {
	t.Helper()
	oldPath := db.NSF_DB_PATH
	oldMode := gin.Mode()
	db.NSF_DB_PATH = dbPath
	gin.SetMode(gin.TestMode)
	f := &serviceTestServer{}
	t.Cleanup(func() {
		f.close()
		db.NSF_DB_PATH = oldPath
		gin.SetMode(oldMode)
	})
	f.start(t)
	return f
}

func (f *serviceTestServer) start(t *testing.T) {
	t.Helper()
	nsfLogger := logger.NewNsfLogger(loggergoUtil.LogLevelString("error"))
	f.server = NewServer(&ServerIE{DbType: "bbolt", NsfLogger: &nsfLogger})
	if f.server.NsfContext == nil {
		t.Fatal("failed to initialize NSF context")
	}
	// Use the actual route registration without opening a fixed TCP port.
	f.server.router = gin.New()
	addService(f.server)
}

func (f *serviceTestServer) close() {
	if f.server != nil && f.server.NsfContext != nil {
		f.server.Processor.Release()
		f.server = nil
	}
}

func (f *serviceTestServer) request(t *testing.T, method, path string, body any, status int) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	f.server.router.ServeHTTP(w, req)
	if w.Code != status {
		t.Fatalf("%s %s: status=%d, want=%d, body=%s", method, path, w.Code, status, w.Body.String())
	}
	return w
}

func (f *serviceTestServer) register(t *testing.T, nf, address string, port int, services ...string) {
	t.Helper()
	w := f.request(t, http.MethodPost, "/sbi/nnsf-nfm", model.NnsfNfmPostRequest{
		NfName: nf, SbiAddress: address, SbiPort: port, Services: services,
	}, http.StatusOK)
	var response model.NnsfNfmPostResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(response.ServiceUuid); err != nil {
		t.Fatalf("registration returned invalid UUID: %q", response.ServiceUuid)
	}
	if response.ServiceToken == "" {
		t.Fatal("registration returned an empty service token")
	}
}

func (f *serviceTestServer) deregister(t *testing.T, nf string, services ...string) {
	t.Helper()
	f.request(t, http.MethodDelete, "/sbi/nnsf-nfm", model.NnsfNfmDeleteRequest{
		NfName: nf, Services: services,
	}, http.StatusNoContent)
}

func (f *serviceTestServer) discover(t *testing.T, want model.NnsfDiscGetResponse) {
	t.Helper()
	w := f.request(t, http.MethodGet, "/sbi/nnsf-disc",
		model.NnsfDiscGetRequest{Service: want.Service}, http.StatusOK)
	var got model.NnsfDiscGetResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("discovery mismatch (-want +got):\n%s", diff)
	}
}

func TestNSFServiceLifecycle(t *testing.T) {
	f := newServiceTestServer(t, filepath.Join(t.TempDir(), "services.db"))
	w := f.request(t, http.MethodGet, "/sbi/heartbeat", nil, http.StatusNoContent)
	if w.Body.Len() != 0 {
		t.Fatal("heartbeat should have no response body")
	}
	f.request(t, http.MethodGet, "/sbi/nnsf-disc",
		model.NnsfDiscGetRequest{Service: "never-registered"}, http.StatusNotFound)
	f.register(t, "NF1", "127.0.0.1", 8080, "service1", "service2")
	f.register(t, "NF2", "127.0.0.2", 8081, "service3")
	for _, want := range []model.NnsfDiscGetResponse{
		{Service: "service1", NfName: "NF1", SbiAddress: "127.0.0.1", SbiPort: 8080},
		{Service: "service2", NfName: "NF1", SbiAddress: "127.0.0.1", SbiPort: 8080},
		{Service: "service3", NfName: "NF2", SbiAddress: "127.0.0.2", SbiPort: 8081},
	} {
		f.discover(t, want)
	}
	f.deregister(t, "NF1", "service1")
	f.request(t, http.MethodGet, "/sbi/nnsf-disc",
		model.NnsfDiscGetRequest{Service: "service1"}, http.StatusNotFound)
	// Removing one service must preserve both its sibling and another NF's service.
	f.discover(t, model.NnsfDiscGetResponse{Service: "service2", NfName: "NF1", SbiAddress: "127.0.0.1", SbiPort: 8080})
	f.discover(t, model.NnsfDiscGetResponse{Service: "service3", NfName: "NF2", SbiAddress: "127.0.0.2", SbiPort: 8081})
	f.deregister(t, "NF1", "service2")
	f.deregister(t, "NF2", "service3")
	for _, service := range []string{"service1", "service2", "service3"} {
		f.request(t, http.MethodGet, "/sbi/nnsf-disc",
			model.NnsfDiscGetRequest{Service: service}, http.StatusNotFound)
	}
}

func TestNSFGroupDiscovery(t *testing.T) {
	f := newServiceTestServer(t, filepath.Join(t.TempDir(), "group.db"))
	f.register(t, "NF1", "127.0.0.1", 8080, "service1", "service2")
	f.register(t, "NF2", "127.0.0.2", 8081, "service3")
	one := model.NnsfDiscGetResponse{Service: "service1", NfName: "NF1", SbiAddress: "127.0.0.1", SbiPort: 8080}
	two := model.NnsfDiscGetResponse{Service: "service2", NfName: "NF1", SbiAddress: "127.0.0.1", SbiPort: 8080}
	three := model.NnsfDiscGetResponse{Service: "service3", NfName: "NF2", SbiAddress: "127.0.0.2", SbiPort: 8081}
	for _, tc := range []struct {
		name     string
		services []string
		want     []model.NnsfDiscGetResponse
	}{
		{name: "same NF", services: []string{"service1", "service2"}, want: []model.NnsfDiscGetResponse{one, two}},
		{name: "multiple NFs and request order", services: []string{"service3", "service1", "service2"}, want: []model.NnsfDiscGetResponse{three, one, two}},
		{name: "single member", services: []string{"service3"}, want: []model.NnsfDiscGetResponse{three}},
		// The following cases document existing processor behavior beyond TA's stated ideal inputs.
		{name: "duplicate members preserved", services: []string{"service1", "service1"}, want: []model.NnsfDiscGetResponse{one, one}},
		{name: "missing members skipped", services: []string{"missing", "service2"}, want: []model.NnsfDiscGetResponse{two}},
		{name: "all missing", services: []string{"missing"}, want: []model.NnsfDiscGetResponse{}},
		{name: "empty group", services: []string{}, want: []model.NnsfDiscGetResponse{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := f.request(t, http.MethodGet, "/sbi/nnsf-disc/group",
				model.NnsfDiscGroupGetRequest{Services: tc.services}, http.StatusOK)
			var got model.NnsfDiscGroupGetResponse
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, got.Services); diff != "" {
				t.Fatalf("group discovery mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNSFRejectsMalformedJSON(t *testing.T) {
	f := newServiceTestServer(t, filepath.Join(t.TempDir(), "malformed.db"))
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/sbi/nnsf-nfm"},
		{http.MethodDelete, "/sbi/nnsf-nfm"},
		{http.MethodGet, "/sbi/nnsf-disc"},
		{http.MethodGet, "/sbi/nnsf-disc/group"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString("{"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			f.server.router.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("malformed JSON: status=%d, want=400, body=%s", w.Code, w.Body.String())
			}
		})
	}
	// Failed input must not prevent later valid requests.
	f.register(t, "NF1", "127.0.0.1", 8080, "valid-service")
	f.discover(t, model.NnsfDiscGetResponse{Service: "valid-service", NfName: "NF1", SbiAddress: "127.0.0.1", SbiPort: 8080})
}

func TestNSFConcurrentServices(t *testing.T) {
	f := newServiceTestServer(t, filepath.Join(t.TempDir(), "concurrent.db"))
	const workers = 16
	var wg sync.WaitGroup
	errors := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			service := fmt.Sprintf("service-%d", i)
			nf := fmt.Sprintf("NF-%d", i)
			ctx := f.server.NsfContext
			if err := ctx.RegisterServices(nf, "127.0.0.1", 8000+i, []string{service}); err != nil {
				errors <- err
				return
			}
			owner, address, port, err := ctx.DiscoverService(service)
			if err != nil || owner != nf || address != "127.0.0.1" || port != 8000+i {
				errors <- fmt.Errorf("unexpected discovery for %s: %s %s %d, err=%v", service, owner, address, port, err)
				return
			}
			if err := ctx.DeregisterServices(nf, []string{service}); err != nil {
				errors <- err
				return
			}
			if _, _, _, err := ctx.DiscoverService(service); err == nil {
				errors <- fmt.Errorf("deleted service %s is still discoverable", service)
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	// The persisted state must agree with the empty in-memory service table.
	f.close()
	f.start(t)
	for i := 0; i < workers; i++ {
		if _, _, _, err := f.server.DiscoverService(fmt.Sprintf("service-%d", i)); err == nil {
			t.Errorf("service-%d reappeared after restart", i)
		}
	}
}
