package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestBrowseAndLaunch(t *testing.T) {
	manifest := []byte(`{"schemaVersion":2,"layers":[]}`)
	model := []byte("model bytes")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer reader" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v2/project/model/manifests/issued":
			w.Write(manifest)
		case "/v2/project/model/tags/list":
			io.WriteString(w, `{"name":"project/model","tags":["issued"]}`)
		case "/v2/project/model/blobs/" + digest(model):
			w.Write(model)
		case "/v2/project/model/blobs/" + digest([]byte("different bytes")):
			w.Write(model)
		case "/v2/_catalog":
			switch r.URL.Query().Get("last") {
			case "":
				w.Header().Set("Link", "<ignored>; rel=\"next\"")
				io.WriteString(w, `{"repositories":["project/model"]}`)
			case "project/model":
				io.WriteString(w, `{"repositories":["project/site"]}`)
			default:
				t.Errorf("unexpected pagination query %s", r.URL.RawQuery)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	config := configuration{
		Registry:            "https://registry.example.test",
		RegistryUpstream:    upstream.URL,
		AuthorizationServer: "https://auth.example.test/realms/subsoil",
		ClientID:            "subsoil-browser",
		Scopes:              []string{"repository:project/model:pull"},
		Applications:        []application{{ID: "editor", Name: "Example Editor", LaunchURL: "https://app.example.test/open"}},
	}
	server, err := newService(config)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.routes()
	send := func(method, path, token, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s returned %d, want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}

	const token = "Bearer reader"
	const files = "/files/project/model"
	response := send("GET", "/files/_catalog?n=1&q=PROJECT", token, "", 200)
	next, _, ok := strings.Cut(response.Header().Get("Link"), ">;")
	if !ok || !strings.Contains(next, "last=project%2Fmodel") {
		t.Fatalf("unexpected catalog link %q", next)
	}
	response = send("GET", strings.TrimPrefix(next, "<"), token, "", 200)
	if strings.TrimSpace(response.Body.String()) != `{"repositories":["project/site"]}` {
		t.Fatalf("unexpected next page: %s", response.Body.String())
	}
	response = send("GET", files+"/tags/list", token, "", 200)
	if strings.TrimSpace(response.Body.String()) != `{"tags":["issued"]}` {
		t.Fatalf("unexpected versions: %s", response.Body.String())
	}
	response = send("GET", files+"/manifests/issued", token, "", 200)
	if response.Header().Get("Docker-Content-Digest") != digest(manifest) || response.Body.String() != string(manifest) {
		t.Fatal("manifest did not match its digest")
	}
	response = send("GET", files+"/blobs/"+url.PathEscape(digest(model)), token, "", 200)
	if response.Body.String() != string(model) {
		t.Fatal("download did not match the requested blob")
	}
	send("GET", files+"/blobs/"+digest([]byte("different bytes")), token, "", 400)
	send("GET", "/files/reviews/model/manifests/issued", token, "", 404)
	send("PUT", "/v2/project/model/manifests/issued", token, "", 404)

	const launch = `{"application_id":"editor","repository":"project/model","reference":"issued","intent":"edit"}`
	send("POST", "/handoff/launches", "", launch, 401)
	send("POST", "/handoff/launches", "Bearer wrong", launch, 401)
	response = send("POST", "/handoff/launches", token, launch, 200)
	var reply map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	target, err := url.Parse(reply["launch_url"])
	if err != nil || target.Scheme != "https" || target.Host != "app.example.test" || target.Path != "/open" {
		t.Fatalf("unexpected launch URL %q", reply["launch_url"])
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(target.Fragment, "handoff="))
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]any
	if err = json.Unmarshal(payload, &actual); err != nil {
		t.Fatal(err)
	}
	expected := map[string]any{
		"protocol": handoffProtocol,
		"source": map[string]any{
			"registry": config.Registry, "repository": "project/model", "digest": digest(manifest),
		},
		"intent": "edit",
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("unexpected launch content: %s", payload)
	}
	if response.Header().Get("Referrer-Policy") != "no-referrer" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("launch response lacked privacy headers")
	}
}
