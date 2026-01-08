package web

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"blogcms/internal/domain"
)

type ctxKey string

const (
	ctxUserIDKey ctxKey = "userID"
	ctxCSRFKey   ctxKey = "csrf"
)

func shortToken(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Placeholder for request-id propagation; kept minimal intentionally.
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func withTimeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if d <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func userIDFromContext(ctx context.Context) (int64, bool) {
	v := ctx.Value(ctxUserIDKey)
	id, ok := v.(int64)
	return id, ok
}

func csrfFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(ctxCSRFKey)
	t, ok := v.(string)
	return t, ok
}

func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := userIDFromContext(r.Context()); !ok {
			http.Redirect(w, r, "/admin/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withSecurityHeaders(cookieSecure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "same-origin")
			w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			// Conservative CSP suitable for server-rendered templates.
			// EasyMDE is loaded from CDN; sources are allowlisted explicitly.
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"base-uri 'self'; "+
					"form-action 'self'; "+
					"frame-ancestors 'none'; "+
					"img-src 'self' data:; "+
					"script-src 'self' 'unsafe-inline' https://unpkg.com; "+
					"style-src 'self' 'unsafe-inline' https://unpkg.com https://maxcdn.bootstrapcdn.com https://use.fontawesome.com; "+
					"font-src 'self' data: https://maxcdn.bootstrapcdn.com https://use.fontawesome.com")
			if cookieSecure {
				// Only meaningful over HTTPS.
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only protect state-changing methods.
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			next.ServeHTTP(w, r)
			return
		}

		expected, ok := csrfFromContext(r.Context())
		if !ok || expected == "" {
			log.Printf("csrf: missing expected token (path=%s)", r.URL.Path)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// Prefer header token for JS clients; fallback to form field for HTML forms.
		got := r.Header.Get("X-CSRF-Token")
		if got == "" {
			// ParseForm parses both query and POST form bodies (including multipart).
			if err := r.ParseForm(); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			got = r.FormValue("csrf_token")
		} else {
			// For diagnostics only: avoid consuming the body here.
			_ = r.FormValue("csrf_token")
		}

		got = strings.TrimSpace(got)
		got = strings.Trim(got, `"`)

		if got == "" || got != expected {
			headOK := r.Header.Get("X-CSRF-Token") != ""
			formOK := r.FormValue("csrf_token") != ""
			log.Printf("csrf: mismatch (path=%s exp=%s got=%s header=%t form=%t)", r.URL.Path, shortToken(expected), shortToken(got), headOK, formOK)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
func writeDomainError(w http.ResponseWriter, err error) {
	switch err {
	case domain.ErrNotFound:
		http.Error(w, "not found", http.StatusNotFound)
	case domain.ErrUnauthorized:
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	case domain.ErrInvalidArgument:
		http.Error(w, "bad request", http.StatusBadRequest)
	default:
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
