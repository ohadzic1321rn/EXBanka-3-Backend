package interbank

import (
	"context"
	"net/http"
)

// partnerContextKey is the unexported context key carrying the
// authenticated *PartnerBank into downstream handlers. Use
// PartnerFromContext to read it.
type partnerContextKey struct{}

// PartnerFromContext returns the *PartnerBank that AuthMiddleware
// attached to this request's context, or nil if the request never
// passed through AuthMiddleware (which means it wasn't authenticated).
func PartnerFromContext(ctx context.Context) *PartnerBank {
	p, _ := ctx.Value(partnerContextKey{}).(*PartnerBank)
	return p
}

// AuthMiddleware enforces X-Api-Key auth on every inter-bank route
// (other than /interbank itself, which does this inline so that the
// idempotence log can still observe replays that arrive with the
// wrong key — useful for forensics). On success, the resolved
// *PartnerBank is placed in the request context.
func AuthMiddleware(registry *Registry, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get(HeaderAPIKey)
		partner := registry.LookupByInboundKey(key)
		if partner == nil {
			writeProblemJSON(w, http.StatusUnauthorized, "unrecognised X-Api-Key")
			return
		}
		ctx := context.WithValue(r.Context(), partnerContextKey{}, partner)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
