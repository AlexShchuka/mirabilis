//go:build integration

package ghauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestGHAuth_DeviceFlow_RealGhSurfacesUserCode(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh binary not found on PATH")
	}

	const wantCode = "WXYZ-7890"
	caFile, cert := writeGitHubStubCert(t)
	stubAddr := startDeviceCodeStub(t, cert, wantCode)
	proxyURL := startConnectProxy(t, stubAddr)

	t.Setenv("HTTPS_PROXY", proxyURL)
	t.Setenv("SSL_CERT_FILE", caFile)
	installDevcontainerExecShim(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	g := New(ctx, &runner.FakeRunner{RepoVal: t.TempDir()}, 80, 24)
	g.linesCh = make(chan string)
	g.doneCh = make(chan error, 1)

	go g.run()

	deadline := time.After(25 * time.Second)
	for g.code == "" || g.url == "" {
		select {
		case line, ok := <-g.linesCh:
			if !ok {
				if g.code == "" || g.url == "" {
					t.Fatalf("gh stream closed before surfacing code/url: code=%q url=%q", g.code, g.url)
				}
			} else {
				g.onLine(line)
			}
		case <-deadline:
			t.Fatalf("timed out before code/url surfaced: code=%q url=%q lines=%v", g.code, g.url, g.lines)
		}
	}

	if g.code != wantCode {
		t.Errorf("user code = %q, want %q", g.code, wantCode)
	}
	if g.url != "https://github.com/login/device" {
		t.Errorf("device URL = %q, want https://github.com/login/device", g.url)
	}
}

func writeGitHubStubCert(t *testing.T) (string, tls.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "github.com stub"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
		DNSNames:              []string{"github.com", "*.github.com"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	caFile := t.TempDir() + "/github_stub_ca.pem"
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		t.Fatal(err)
	}
	return caFile, tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func startDeviceCodeStub(t *testing.T, cert tls.Certificate, userCode string) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login/device/code", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"device_code":"DC","user_code":%q,"verification_uri":"https://github.com/login/device","expires_in":900,"interval":5}`, userCode)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: mux, TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}}}
	go srv.ServeTLS(ln, "", "")
	t.Cleanup(func() { _ = srv.Close() })
	return ln.Addr().String()
}

func startConnectProxy(t *testing.T, target string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "only CONNECT", http.StatusMethodNotAllowed)
			return
		}
		dst, err := net.Dial("tcp", target)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "no hijack", http.StatusInternalServerError)
			return
		}
		cli, _, err := hj.Hijack()
		if err != nil {
			_ = dst.Close()
			return
		}
		_, _ = cli.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		go pipeConn(dst, cli)
		go pipeConn(cli, dst)
	})}
	go srv.Serve(ln)
	t.Cleanup(func() { _ = srv.Close() })
	return "http://" + ln.Addr().String()
}

func pipeConn(dst, src net.Conn) {
	defer func() { _ = dst.Close() }()
	defer func() { _ = src.Close() }()
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func installDevcontainerExecShim(t *testing.T) {
	t.Helper()
	shimDir := t.TempDir()
	body := "#!/bin/sh\nshift 3\nexec \"$@\"\n"
	if err := os.WriteFile(shimDir+"/devcontainer", []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+":"+os.Getenv("PATH"))
}
