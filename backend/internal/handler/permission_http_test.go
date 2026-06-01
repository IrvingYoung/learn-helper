package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPermissionResponseHandler_NotFound(t *testing.T) {
	h := &AIHandler{permissions: NewPermissionRegistry()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ai/permission_response", h.HandlePermissionResponse)

	body := `{"request_id":"unknown","decisions":[]}`
	req := httptest.NewRequest("POST", "/api/ai/permission_response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (unknown id is a no-op, not an error)", rr.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status field = %v, want ok", resp["status"])
	}
}

func TestPermissionResponseHandler_DeliversDecision(t *testing.T) {
	h := &AIHandler{permissions: NewPermissionRegistry()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ai/permission_response", h.HandlePermissionResponse)

	ch := h.permissions.Register("perm-1", 5)

	body := `{"request_id":"perm-1","decisions":[{"id":"toolu_x","action":"approve"}]}`
	req := httptest.NewRequest("POST", "/api/ai/permission_response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(rr, req)
		close(done)
	}()

	select {
	case got := <-ch:
		if len(got) != 1 || got[0].ID != "toolu_x" || got[0].Action != "approve" {
			t.Errorf("got %+v, want one approve decision for toolu_x", got)
		}
	case <-time.After(time.Second):
		t.Fatal("decisions not delivered within 1s")
	}
	<-done

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}
