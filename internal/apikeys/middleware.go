package apikeys

import (
	"net/http"
	"strings"

	"github.com/johandavid77/btrfs-snapah-pow/internal/auth"
)

func CombinedMiddleware(jwtMgr *auth.Manager, store *Store, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get("Authorization")
		apiKeyHeader := r.Header.Get("X-API-Key")

		rawKey := apiKeyHeader
		if rawKey == "" && strings.HasPrefix(bearer, "Bearer spow_") {
			rawKey = strings.TrimPrefix(bearer, "Bearer ")
		}

		if rawKey != "" {
			k, valid := store.Validate(rawKey)
			if !valid {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"API key invalida o expirada"}`))
				return
			}
			r.Header.Set("X-User-ID", k.ID)
			r.Header.Set("X-Username", k.Name)
			r.Header.Set("X-User-Role", k.Role)
			next(w, r)
			return
		}

		jwtMgr.Middleware(next)(w, r)
	}
}
