package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

type launchRequest struct {
	ApplicationID string `json:"application_id"`
	Repository    string `json:"repository"`
	Reference     string `json:"reference"`
	Intent        string `json:"intent"`
	Layer         *int   `json:"layer,omitempty"`
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

func validIntent(intent string) bool {
	return intent == "view" || intent == "edit"
}

func parseLayer(query url.Values) (*int, *apiError) {
	if !query.Has("layer") {
		return nil, nil
	}
	index, err := strconv.Atoi(query.Get("layer"))
	if err != nil || index < 0 || len(query["layer"]) != 1 {
		return nil, badRequest("Choose a file from this release")
	}
	return &index, nil
}

func (s *service) availableApps(ctx context.Context, token, repo string) ([]application, *apiError) {
	if !strings.HasPrefix(repo, "workspaces/") {
		return s.config.Applications, nil
	}
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
	registrations, err := s.listWorkspaceApps(ctx, owner)
	if err != nil {
		return nil, unavailable()
	}
	var apps []application
	for _, app := range registrations {
		if app.LaunchURL != "" {
			apps = append(apps, application{
				ID: app.ID, Name: app.Name, LaunchURL: app.LaunchURL, Handles: app.Handles, FileHandles: app.FileHandles,
			})
		}
	}
	return apps, nil
}

func matchingApps(apps []application, manifest releaseManifest, layer *int) ([]appMatch, *apiError) {
	if layer != nil && (*layer < 0 || *layer >= len(manifest.Layers)) {
		return nil, badRequest("Choose a file from this release")
	}
	var matches []appMatch
	for _, app := range apps {
		match := appMatch{application: app}
		if layer == nil {
			for _, entry := range app.Handles {
				if entry.ArtifactType == manifest.ArtifactType {
					match.intents = append(match.intents, entry.Intents...)
				}
			}
		} else {
			for _, entry := range app.FileHandles {
				if entry.MediaType == manifest.Layers[*layer].MediaType {
					match.intents = append(match.intents, entry.Intents...)
				}
			}
		}
		if len(match.intents) > 0 {
			slices.Sort(match.intents)
			match.intents = slices.Compact(match.intents)
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
	layer, failure := parseLayer(query)
	if failure != nil {
		writeJSON(w, failure.status, failure)
		return
	}
	fingerprint, matches, failure := s.launchMatches(r.Context(), token, query.Get("repository"), query.Get("reference"), layer)
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
	if err := decodeJSON(http.MaxBytesReader(w, r.Body, 32<<10), &req); err != nil {
		failure := badRequest("Choose a registered application, version and intent")
		writeJSON(w, failure.status, failure)
		return
	}
	target, failure := s.launchURL(r.Context(), token, req)
	if failure != nil {
		writeJSON(w, failure.status, failure)
		return
	}
	w.Header().Set("Referrer-Policy", "no-referrer")
	writeJSON(w, http.StatusOK, map[string]string{"protocol": handoffProtocol, "launch_url": target})
}

func (s *service) launchMatches(ctx context.Context, token, repository, ref string, layer *int) (string, []appMatch, *apiError) {
	fingerprint, manifest, failure := s.readRelease(ctx, token, repository, ref)
	if failure != nil {
		return "", nil, failure
	}
	apps, failure := s.availableApps(ctx, token, repository)
	if failure != nil {
		return "", nil, failure
	}
	matches, failure := matchingApps(apps, manifest, layer)
	return fingerprint, matches, failure
}

func (s *service) launchURL(ctx context.Context, token string, req launchRequest) (string, *apiError) {
	if !validIntent(req.Intent) || req.ApplicationID == "" {
		return "", badRequest("Choose a registered application, version and intent")
	}
	if req.Layer != nil && !digestPattern.MatchString(req.Reference) {
		return "", badRequest("Choose a file from an exact release")
	}
	fingerprint, matches, failure := s.launchMatches(ctx, token, req.Repository, req.Reference, req.Layer)
	if failure != nil {
		return "", failure
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
		if req.Layer != nil {
			target.RawQuery = url.Values{"layer": {strconv.Itoa(*req.Layer)}}.Encode()
		}
		return target.String(), nil
	}
	return "", badRequest("Choose an app that supports this item and action")
}
