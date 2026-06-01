package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAskUserResponseHandler_MissingID(t *testing.T) {
	h := &AIHandler{askUsers: NewAskUserRegistry()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ai/ask_user_response", h.HandleAskUserResponse)

	body := `{"request_id":"","answer":"x"}`
	req := httptest.NewRequest("POST", "/api/ai/ask_user_response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestAskUserResponseHandler_UnknownRequestIDReturnsOK(t *testing.T) {
	h := &AIHandler{askUsers: NewAskUserRegistry()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ai/ask_user_response", h.HandleAskUserResponse)

	body := `{"request_id":"unknown","answer":"底层原理"}`
	req := httptest.NewRequest("POST", "/api/ai/ask_user_response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (unknown id is a no-op)", rr.Code)
	}
}

func TestAskUserResponseHandler_DeliversAnswer(t *testing.T) {
	h := &AIHandler{askUsers: NewAskUserRegistry()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ai/ask_user_response", h.HandleAskUserResponse)

	ch := h.askUsers.Register("ask-1")

	body := `{"request_id":"ask-1","answer":"底层原理"}`
	req := httptest.NewRequest("POST", "/api/ai/ask_user_response", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(rr, req)
		close(done)
	}()

	select {
	case got := <-ch:
		if got.Answer != "底层原理" {
			t.Errorf("Answer = %v, want 底层原理", got.Answer)
		}
	case <-time.After(time.Second):
		t.Fatal("answer not delivered within 1s")
	}
	<-done

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
}
