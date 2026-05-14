package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_On_Off(t *testing.T) {
	var lastPath, lastMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		lastPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.On(context.Background(), "abc"); err != nil {
		t.Fatal(err)
	}
	if lastMethod != "POST" || lastPath != "/bulb/abc/on" {
		t.Fatalf("got %s %s", lastMethod, lastPath)
	}
	if err := c.Off(context.Background(), "abc"); err != nil {
		t.Fatal(err)
	}
	if lastPath != "/bulb/abc/off" {
		t.Fatalf("off path: %s", lastPath)
	}
}

func TestClient_Brightness_Body(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.Brightness(context.Background(), "abc", 42); err != nil {
		t.Fatal(err)
	}
	if got["level"].(float64) != 42 {
		t.Fatalf("level: %v", got["level"])
	}
}

func TestClient_List_Decode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"bulbs":[{"mac":"abc","name":"bedroom","ip":"10.0.0.1"}]}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	out, err := c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].MAC != "abc" || out[0].Name != "bedroom" {
		t.Fatalf("got %+v", out)
	}
}

func TestClient_DefaultTarget_PassesThrough(t *testing.T) {
	var lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	if err := c.On(context.Background(), "_default"); err != nil {
		t.Fatal(err)
	}
	if lastPath != "/bulb/_default/on" {
		t.Fatalf("got %s", lastPath)
	}
}
