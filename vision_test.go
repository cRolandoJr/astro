package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHttpVisionMandaImagenYParsea(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(img, []byte{0x89, 0x50, 0x4e, 0x47}, 0o644); err != nil {
		t.Fatal(err)
	}
	var gotBody, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"Veo una terminal con un error."}}]}`)
	}))
	defer srv.Close()

	ans, err := httpVision(srv.URL, "secreto", "gemini-flash-latest")("¿qué ves?", img)
	if err != nil {
		t.Fatal(err)
	}
	if ans != "Veo una terminal con un error." {
		t.Fatalf("respuesta mal parseada: %q", ans)
	}
	if gotAuth != "Bearer secreto" {
		t.Fatalf("faltó Authorization: %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"type":"image_url"`) || !strings.Contains(gotBody, "data:image/png;base64,") {
		t.Fatalf("el body debía incluir la imagen base64; fue %s", gotBody)
	}
	if !strings.Contains(gotBody, "¿qué ves?") {
		t.Fatalf("el body debía incluir la pregunta; fue %s", gotBody)
	}
}

func TestHttpVisionErrorSiNoHayImagen(t *testing.T) {
	if _, err := httpVision("http://x", "k", "m")("q", "/no/existe.png"); err == nil {
		t.Fatal("esperaba error si la imagen no existe")
	}
}

func TestHttpVisionErrorHTTP(t *testing.T) {
	dir := t.TempDir()
	img := filepath.Join(dir, "s.png")
	if err := os.WriteFile(img, []byte{1}, 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := httpVision(srv.URL, "k", "m")("q", img); err == nil {
		t.Fatal("esperaba error en HTTP 403")
	}
}
