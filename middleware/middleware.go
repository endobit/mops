package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// Chain is a helper function to chain multiple middleware functions together.
// Middleware functions will be called in the order they are passed to Chain.
func Chain(mw ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		for i := len(mw) - 1; i >= 0; i-- {
			h = mw[i](h)
		}
		return h
	}
}

// RequestID generates a request ID for each request and attaches it
// to the context and response headers.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint64(&requestID, 1)

		w.Header().Set("X-Request-Id", fmt.Sprint(requestID))
		ctx := context.WithValue(r.Context(), ctxKeyRequestID{}, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Recovery recovers from panics and logs the error. It will return a
// 500 Internal Server Error to the client.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered", "error", rec)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// Logging logs the request data and latency of the call. It will log
// after the handler has completed.
func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t0 := time.Now()
			rw := &responseWriter{ResponseWriter: w}

			defer func() { // still runs even if panic occurs
				level := slog.LevelInfo

				resp := []any{
					slog.Int("status", rw.code),
					slog.Duration("duration", time.Since(t0)),
					slog.Int("bytes", rw.length),
				}

				if rw.code >= 400 {
					level = slog.LevelError
					resp = append(resp, slog.String("error", rw.errMessage))
				}

				logger.LogAttrs(context.Background(), level, "handler",
					slog.Group("call",
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.Any("query", r.URL.Query()),
						slog.Int("bytes", int(r.ContentLength))),
					slog.Group("result", resp...),
					slog.Group("client",
						slog.String("address", r.RemoteAddr),
						slog.String("user-agent", r.Header.Get("User-Agent"))),
					slog.Uint64("id", getRequestID(r.Context())))

			}()

			next.ServeHTTP(rw, r)
		})
	}
}

// DefaultJSON sets the Content-Type to JSON if it has not already
// been set.
func DefaultJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if w.Header().Get("Content-Type") == "" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
			}
		}()

		next.ServeHTTP(w, r)
	})
}

type ctxKeyRequestID struct{}

// responseWriter wraps http.ResponseWriter to capture the status code and the
// size of the data written
type responseWriter struct {
	http.ResponseWriter
	code       int
	length     int
	written    bool
	errMessage string
}

var requestID uint64

func (r *responseWriter) WriteHeader(code int) {
	if !r.written {
		r.written = true
		r.code = code
	}

	r.ResponseWriter.WriteHeader(code)
}

func (r *responseWriter) Write(b []byte) (int, error) {
	if !r.written {
		r.WriteHeader(http.StatusOK)
	}

	n, err := r.ResponseWriter.Write(b)
	r.length += n

	if r.code >= 400 { // on error the body is an error message
		r.errMessage = string(b)
	}

	return n, err
}

// getRequestID is a helper to fetch the request ID from the context.
func getRequestID(ctx context.Context) uint64 {
	if id, ok := ctx.Value(ctxKeyRequestID{}).(uint64); ok {
		return id
	}
	return 0
}
