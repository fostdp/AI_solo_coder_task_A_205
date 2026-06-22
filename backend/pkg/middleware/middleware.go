package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

var gzPool = sync.Pool{
	New: func() interface{} {
		w := gzip.NewWriter(io.Discard)
		return w
	},
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
	http.Hijacker
	http.Flusher
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if w.Flusher != nil {
		w.Flusher.Flush()
	}
	if fw, ok := w.Writer.(interface{ Flush() }); ok {
		fw.Flush()
	}
}

func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzPool.Get().(*gzip.Writer)
		defer gzPool.Put(gz)

		gz.Reset(w)

		hijacker, _ := w.(http.Hijacker)
		flusher, _ := w.(http.Flusher)

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length")

		gw := &gzipResponseWriter{
			Writer:         gz,
			ResponseWriter: w,
			Hijacker:       hijacker,
			Flusher:        flusher,
		}

		next.ServeHTTP(gw, r)

		gz.Close()
	})
}

type CORSHandler struct {
	handler http.Handler
	origin  string
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

type LoggingHandler struct {
	handler http.Handler
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/ws") && !strings.HasPrefix(r.URL.Path, "/debug") {
		}
		next.ServeHTTP(w, r)
	})
}
