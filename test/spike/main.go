package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"
)

const oauthBeta = "oauth-2025-04-20"

func main() {
	listen := flag.String("listen", "127.0.0.1:8788", "host proxy listen address")
	injectBeta := flag.Bool("inject-beta", false, "ensure anthropic-beta includes "+oauthBeta)
	sessionKey := flag.String("session-key", "", "session key the container must present")
	flag.Parse()

	if *sessionKey == "" {
		log.Fatal("spike: -session-key required")
	}
	token, err := keychainToken()
	if err != nil {
		log.Fatalf("spike: token: %v", err)
	}
	if !strings.HasPrefix(token, "sk-ant-oat01-") {
		log.Fatalf("spike: keychain entry is not an oat token (prefix %q)", prefix(token, 12))
	}

	upstream := &url.URL{Scheme: "https", Host: "api.anthropic.com"}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(upstream)
			pr.Out.Host = upstream.Host
			pr.Out.Header.Set("Authorization", "Bearer "+token)
			if *injectBeta {
				ensureBeta(pr.Out.Header)
			}
		},
		FlushInterval: -1,
		ErrorLog:      log.New(os.Stderr, "rp: ", log.LstdFlags),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+*sessionKey {
			log.Printf("REJECT %s %s (bad session key, auth-prefix=%q)", r.Method, r.URL.Path, prefix(auth, 14))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		beta := r.Header.Get("anthropic-beta")
		if beta == "" {
			beta = "<absent>"
		}
		ua := r.Header.Get("User-Agent")
		sw := &statusWriter{ResponseWriter: w}
		rp.ServeHTTP(sw, r)
		log.Printf("%s %s -> %d in %s | client-beta=%s | ua=%q | inject=%v",
			r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond), beta, ua, *injectBeta)
	})

	log.Printf("spike proxy on %s -> %s (inject-beta=%v)", *listen, upstream.Host, *injectBeta)
	log.Fatal(http.ListenAndServe(*listen, mux))
}

func keychainToken() (string, error) {
	account := os.Getenv("USER")
	if account == "" {
		account = "mirabilis"
	}
	out, err := exec.Command("security", "find-generic-password", "-a", account, "-s", "mirabilis-claude-token", "-w").Output()
	if err != nil {
		return "", fmt.Errorf("keychain mirabilis-claude-token: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func ensureBeta(h http.Header) {
	cur := h.Get("anthropic-beta")
	if strings.Contains(cur, oauthBeta) {
		return
	}
	if cur == "" {
		h.Set("anthropic-beta", oauthBeta)
		return
	}
	h.Set("anthropic-beta", cur+","+oauthBeta)
}

func prefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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
