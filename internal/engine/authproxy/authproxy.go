package authproxy

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime"
	"strconv"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/claudeauth"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

const (
	node              = "proxy"
	shutdownTimeout   = 3 * time.Second
	tokenResolveLimit = 5 * time.Second
	readHeaderTimeout = 15 * time.Second
	readTimeout       = 60 * time.Second
	idleTimeout       = 120 * time.Second
)

type Proxy struct {
	ts       claudeauth.TokenSource
	obs      *obs.Obs
	log      *slog.Logger
	upstream *url.URL
	done     chan struct{}
	key      string
	addr     string
	port     int
}

func New(ts claudeauth.TokenSource, o *obs.Obs, port int, key string) *Proxy {
	if key == "" {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			panic(fmt.Sprintf("authproxy: session key: %v", err))
		}
		key = hex.EncodeToString(buf)
	}
	return &Proxy{
		ts:       ts,
		obs:      o,
		log:      o.Logger("authproxy"),
		upstream: &url.URL{Scheme: "https", Host: "api.anthropic.com"},
		done:     make(chan struct{}),
		key:      key,
		port:     port,
	}
}

func (p *Proxy) Key() string { return p.key }

func (p *Proxy) Addr() string { return p.addr }

func (p *Proxy) Start(ctx context.Context) error {
	host := bindHost(runtime.GOOS)
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(p.port)))
	if err != nil {
		p.obs.Set(node, obs.StateDegraded, "listen failed")
		return fmt.Errorf("authproxy: listen: %w", err)
	}
	p.addr = ln.Addr().String()
	_, boundPort, _ := net.SplitHostPort(p.addr)
	p.obs.Set(node, obs.StateOK, "listening :"+boundPort)
	p.log.Info("listening", slog.String("addr", p.addr), slog.String("interface", host))

	srv := &http.Server{
		Handler:           p.handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
	}
	served := make(chan struct{})
	go func() {
		defer close(served)
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			p.log.Info("serve stopped", slog.Any("error", err))
		}
	}()
	go func() {
		defer close(p.done)
		select {
		case <-ctx.Done():
		case <-served:
		}
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			_ = srv.Close()
		}
		<-served
		p.obs.Set(node, obs.StateOff, "stopped")
	}()
	return nil
}

type tokenKey struct{}

func (p *Proxy) handler() http.Handler {
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(p.upstream)
			pr.Out.Host = p.upstream.Host
			pr.Out.Header.Del("X-Api-Key")
			pr.Out.Header.Del("X-Forwarded-For")
			pr.Out.Header.Del("X-Forwarded-Host")
			pr.Out.Header.Del("X-Forwarded-Proto")
			if tok, ok := pr.In.Context().Value(tokenKey{}).(string); ok {
				pr.Out.Header.Set("Authorization", "Bearer "+tok)
			}
		},
		FlushInterval: -1,
		ErrorLog:      slog.NewLogLogger(p.log.Handler(), slog.LevelInfo),
	}
	want := []byte("Bearer " + p.key)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			p.log.Info("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.status),
				slog.Duration("duration", time.Since(start)),
				slog.Bool("client_beta", r.Header.Get("anthropic-beta") != ""),
			)
		}()
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(sw, "unauthorized", http.StatusUnauthorized)
			return
		}
		tokenCtx, cancel := context.WithTimeout(r.Context(), tokenResolveLimit)
		defer cancel()
		token, err := p.ts.Token(tokenCtx)
		if err != nil || token == "" {
			p.obs.Set(node, obs.StateDegraded, "token not ready")
			p.log.Info("token not ready", slog.Bool("has_err", err != nil))
			http.Error(sw, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		rp.ServeHTTP(sw, r.WithContext(context.WithValue(r.Context(), tokenKey{}, token)))
		if sw.status == http.StatusUnauthorized {
			if inv, ok := p.ts.(interface{ Invalidate() }); ok {
				inv.Invalidate()
			}
			p.obs.Set(node, obs.StateDegraded, "upstream rejected token")
			return
		}
		p.obs.Set(node, obs.StateOK, "ok")
	})
}

func bindHost(goos string) string {
	if goos == "linux" {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
