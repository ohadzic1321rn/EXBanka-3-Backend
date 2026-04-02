package handler

import (
	"encoding/json"
	"net/http"
	"strings"
)

// CombinedTransferHandler routes transfer requests to the appropriate handler.
type CombinedTransferHandler struct {
	http     *TransferHTTPHandler
	mobile   *TransferMobileVerificationHandler
	fallback http.Handler
}

func NewCombinedTransferHandler(httpHandler *TransferHTTPHandler, mobile *TransferMobileVerificationHandler, fallback http.Handler) *CombinedTransferHandler {
	return &CombinedTransferHandler{http: httpHandler, mobile: mobile, fallback: fallback}
}

func (h *CombinedTransferHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/v1/transfers" && r.Method == http.MethodPost {
		h.http.Create(w, r)
		return
	}
	if r.URL.Path == "/api/v1/transfers/preview" && r.Method == http.MethodPost {
		h.http.Preview(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v1/transfers/client/") {
		h.http.ListByClient(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v1/transfers/account/") {
		h.http.ListByAccount(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/approve") {
		h.mobile.Approve(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reject") {
		h.mobile.Reject(w, r)
		return
	}
	h.fallback.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, statusCode int, payload map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
