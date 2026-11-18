package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
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

type appMatch struct {
	application
	intents []string
}

func (s *service) matchingApps(ctx context.Context, token, repo string, manifest []byte) ([]appMatch, *apiError) {
	var source struct {
		ArtifactType string `json:"artifactType"`
	}
	if json.Unmarshal(manifest, &source) != nil {
		return nil, unavailable()
	}
	if source.ArtifactType == "" {
		return nil, nil
	}
	apps := s.config.Applications
	if strings.HasPrefix(repo, "workspaces/") {
		apps = nil
		if s.identity == nil {
			return nil, nil
		}
		owner, err := s.identity.owner(ctx, token)
		if err == noWorkspace || (err == nil && workspaceRepository(owner) != repo) {
			return nil, nil
		}
		if err != nil {
			return nil, unavailable()
		}
		admin, err := s.identity.adminToken(ctx)
		if err != nil {
			return nil, unavailable()
		}
		registrations, err := s.identity.workspaceApps(ctx, owner, admin)
		if err != nil {
			return nil, unavailable()
		}
		for _, entry := range registrations {
			if app, ok := s.copyableApp(owner, entry); ok && app.LaunchURL != "" {
				apps = append(apps, application{
					ID: app.ID, Name: app.Name, LaunchURL: app.LaunchURL, Handles: app.Handles,
				})
			}
		}
	}
	var matches []appMatch
	for _, app := range apps {
		match := appMatch{application: app}
		for _, entry := range app.Handles {
			if entry.ArtifactType != source.ArtifactType {
				continue
			}
			for _, intent := range entry.Intents {
				if !slices.Contains(match.intents, intent) {
					match.intents = append(match.intents, intent)
				}
			}
		}
		if len(match.intents) > 0 {
			matches = append(matches, match)
		}
	}
	return matches, nil
}

func (s *service) applications(w http.ResponseWriter, r *http.Request) {
	token := authorization(w, r)
	if token == "" {
		return
	}
	query := r.URL.Query()
	fingerprint, manifest, failure := s.sourceManifest(r.Context(), token, query.Get("repository"), query.Get("reference"))
	if failure != nil {
		writeJSON(w, failure.status, failure)
		return
	}

	matches, failure := s.matchingApps(r.Context(), token, query.Get("repository"), manifest)
	if failure != nil {
		writeJSON(w, failure.status, failure)
		return
	}
	apps := make([]map[string]any, 0, len(matches))
	for _, app := range matches {
		launches := map[string]map[string]string{}
		for _, intent := range app.intents {
			launches[intent] = map[string]string{"intent": intent}
		}
		apps = append(apps, map[string]any{
			"id": app.ID, "name": app.Name,
			"launches": launches,
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
	fingerprint, manifest, failure := s.sourceManifest(r.Context(), token, req.Repository, req.Reference)
	if failure != nil {
		writeJSON(w, failure.status, failure)
		return
	}
	matches, failure := s.matchingApps(r.Context(), token, req.Repository, manifest)
	if failure != nil {
		writeJSON(w, failure.status, failure)
		return
	}
	for _, app := range matches {
		if app.ID != req.ApplicationID || !slices.Contains(app.intents, req.Intent) {
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
	failure = badRequest("Choose an app that supports this version and action")
	writeJSON(w, failure.status, failure)
}
