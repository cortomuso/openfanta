package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type middleware func(http.Handler) http.Handler

// chain applies middleware so that the first one listed is the outermost.
func chain(h http.Handler, mws ...middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

func recoverer(log *slog.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					log.ErrorContext(r.Context(), "panic serving request", "path", r.URL.Path, "panic", v)
					writeProblem(w, http.StatusInternalServerError, "internal error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func requestLogger(log *slog.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.InfoContext(r.Context(), "request",
				"method", r.Method, "path", r.URL.Path, "status", rec.status, "duration_ms", time.Since(start).Milliseconds())
		})
	}
}

func timeout(d time.Duration) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bufferedWriter captures a response so its ETag can be computed.
type bufferedWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedWriter) Header() http.Header         { return b.header }
func (b *bufferedWriter) WriteHeader(code int)        { b.status = code }
func (b *bufferedWriter) Write(p []byte) (int, error) { return b.body.Write(p) }

// etag adds a strong ETag and caching headers to successful GET responses and
// answers matching If-None-Match requests with 304 Not Modified.
func etag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		buf := &bufferedWriter{header: http.Header{}, status: http.StatusOK}
		next.ServeHTTP(buf, r)
		for k, v := range buf.header {
			w.Header()[k] = v
		}
		if buf.status != http.StatusOK {
			w.WriteHeader(buf.status)
			_, _ = w.Write(buf.body.Bytes())
			return
		}
		sum := sha256.Sum256(buf.body.Bytes())
		tag := `"` + hex.EncodeToString(sum[:16]) + `"`
		w.Header().Set("ETag", tag)
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			w.Header().Set("Cache-Control", "public, max-age=60")
		} else {
			w.Header().Set("Cache-Control", "no-store")
		}
		if matches(r.Header.Get("If-None-Match"), tag) {
			w.Header().Del("Content-Type")
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.body.Bytes())
	})
}

func matches(ifNoneMatch, tag string) bool {
	for _, candidate := range strings.Split(ifNoneMatch, ",") {
		c := strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if c == tag || c == "*" {
			return true
		}
	}
	return false
}
