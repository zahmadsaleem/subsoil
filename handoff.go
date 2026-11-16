package main

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
)

type launchRequest struct {
	ApplicationID string `json:"application_id"`
	Repository    string `json:"repository"`
	Reference     string `json:"reference"`
	Intent        string `json:"intent"`
}

type source struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Digest     string `json:"digest"`
}

type handoff struct {
	Protocol string `json:"protocol"`
	Source   source `json:"source"`
	Intent   string `json:"intent"`
}

func (s *service) applications(w http.ResponseWriter, r *http.Request) {
	token := authorization(w, r)
	if token == "" {
		return
	}
	query := r.URL.Query()
	fingerprint, _, failure := s.sourceManifest(r.Context(), token, query.Get("repository"), query.Get("reference"))
	if failure != nil {
		writeJSON(w, failure.status, failure)
		return
	}

	apps := make([]map[string]any, 0, len(s.config.Applications))
	for _, app := range s.config.Applications {
		apps = append(apps, map[string]any{
			"id": app.ID, "name": app.Name,
			"launches": map[string]map[string]string{"view": {"intent": "view"}, "edit": {"intent": "edit"}},
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"protocol": handoffProtocol, "digest": fingerprint, "applications": apps,
	})
}

func (s *service) launch(w http.ResponseWriter, r *http.Request) {
	token := authorization(w, r)
	if token == "" {
		return
	}
	var req launchRequest
	if err := decodeJSON(http.MaxBytesReader(w, r.Body, 32<<10), &req); err != nil || (req.Intent != "view" && req.Intent != "edit") {
		failure := badRequest("Choose a registered application, version and intent")
		writeJSON(w, failure.status, failure)
		return
	}
	fingerprint, _, failure := s.sourceManifest(r.Context(), token, req.Repository, req.Reference)
	if failure != nil {
		writeJSON(w, failure.status, failure)
		return
	}
	for _, app := range s.config.Applications {
		if app.ID != req.ApplicationID {
			continue
		}
		target, _ := url.Parse(app.LaunchURL) // Checked when loading the configuration.
		data, _ := json.Marshal(handoff{
			Protocol: handoffProtocol,
			Source:   source{Registry: s.config.Registry, Repository: req.Repository, Digest: fingerprint},
			Intent:   req.Intent,
		})
		target.Fragment = "handoff=" + base64.RawURLEncoding.EncodeToString(data)
		w.Header().Set("Referrer-Policy", "no-referrer")
		writeJSON(w, http.StatusOK, map[string]string{"protocol": handoffProtocol, "launch_url": target.String()})
		return
	}
	failure = badRequest("Choose a registered application")
	writeJSON(w, failure.status, failure)
}
