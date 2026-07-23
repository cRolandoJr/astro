package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHttpEmbedParseaYAutoriza(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		fmt.Fprint(w, `{"data":[{"embedding":[0.1,0.2,0.3]}]}`)
	}))
	defer srv.Close()

	vec, err := httpEmbed(srv.URL, "secreto", "text-embedding-004")("hola")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 3 || vec[0] != 0.1 {
		t.Fatalf("vector mal parseado: %v", vec)
	}
	if gotAuth != "Bearer secreto" {
		t.Fatalf("faltó Authorization: %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"input":"hola"`) || !strings.Contains(gotBody, `"model":"text-embedding-004"`) {
		t.Fatalf("body inesperado: %s", gotBody)
	}
}

func TestHttpEmbedErrorHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := httpEmbed(srv.URL, "k", "m")("x"); err == nil {
		t.Fatal("esperaba error en HTTP 403")
	}
}
