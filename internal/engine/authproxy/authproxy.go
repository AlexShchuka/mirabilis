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
	node            = "proxy"
	shutdownTimeout = 3 * time.Second
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

func New(ts claudeauth.TokenSource, o *obs.Obs, port int) *Proxy {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("authproxy: session key: %v", err))
	}
	return &Proxy{
		ts:       ts,
		obs:      o,
		log:      o.Logger("authproxy"),
		upstream: &url.URL{Scheme: "https", Host: "api.anthropic.com"},
		done:     make(chan struct{}),
		key:      hex.EncodeToString(buf),
		port:     port,
	}
}

func (p *Proxy) Key() string { return p.key }

func (p *Proxy) Addr() string { return p.addr }

func (p *Proxy) Start(ctx context.Context) error {
	token, err := p.ts.Token(ctx)
	if err != nil {
		p.obs.Set(node, obs.StateDegraded, "token unavailable")
		return fmt.Errorf("authproxy: token: %w", err)
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(bindHost(runtime.GOOS), strconv.Itoa(p.port)))
	if err != nil {
		p.obs.Set(node, obs.StateDegraded, "listen failed")
		return fmt.Errorf("authproxy: listen: %w", err)
	}
	p.addr = ln.Addr().String()
	_, boundPort, _ := net.SplitHostPort(p.addr)
	p.obs.Set(node, obs.StateOK, "listening :"+boundPort)
	p.log.Info("listening", slog.String("addr", p.addr))

	srv := &http.Server{Handler: p.handler(token)}
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

func (p *Proxy) handler(token string) http.Handler {
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(p.upstream)
			pr.Out.Host = p.upstream.Host
			pr.Out.Header.Set("Authorization", "Bearer "+token)
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
		rp.ServeHTTP(sw, r)
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
