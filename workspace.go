package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode"
)

const workspaceAppLimit = 8

var errAppExists = errors.New("app already registered")

type capability struct {
	ArtifactType string   `json:"artifact_type"`
	Intents      []string `json:"intents"`
}

type appRegistration struct {
	Name         string       `json:"name"`
	RedirectURIs []string     `json:"redirect_uris"`
	LaunchURL    string       `json:"launch_url,omitempty"`
	Handles      []capability `json:"handles"`
}

type registeredApp struct {
	appRegistration
	ID        string `json:"id"`
	ClientID  string `json:"client_id"`
	Issuer    string `json:"issuer"`
	Registry  string `json:"registry"`
	Workspace string `json:"repository"`
}

type keycloakApp struct {
	ID         string            `json:"id,omitempty"`
	ClientID   string            `json:"clientId"`
	Attributes map[string]string `json:"attributes"`
	Enabled    bool              `json:"enabled"`
}

func workspaceRepository(owner string) string {
	return "workspaces/" + owner + "/scratch"
}

func (a appRegistration) validate() error {
	if a.Name == "" || len(a.Name) > 80 || strings.ContainsFunc(a.Name, unicode.IsControl) {
		return errors.New("Enter an app name up to 80 characters")
	}
	if len(a.RedirectURIs) == 0 || len(a.RedirectURIs) > 4 {
		return errors.New("Enter between one and four callback URLs")
	}
	for _, address := range a.RedirectURIs {
		if !localAppURL(address) {
			return errors.New("Use exact callback URLs on " + developerOrigin)
		}
	}
	if a.LaunchURL != "" && !localAppURL(a.LaunchURL) {
		return errors.New("Use an Open with address on " + developerOrigin)
	}
	return validateHandles(a.Handles)
}

func localAppURL(address string) bool {
	u, err := originURL(address, false)
	return err == nil && u.Scheme+"://"+u.Host == developerOrigin &&
		strings.HasPrefix(u.Path, "/") && !strings.ContainsAny(u.Path, "*\\")
}

func validateHandles(handles []capability) error {
	if len(handles) == 0 || len(handles) > 12 {
		return errors.New("Choose between one and twelve artifact types")
	}
	for _, handle := range handles {
		mediaType, params, err := mime.ParseMediaType(handle.ArtifactType)
		if err != nil || len(params) != 0 || mediaType != handle.ArtifactType ||
			strings.Count(mediaType, "/") != 1 || strings.Contains(mediaType, "*") || len(mediaType) > 128 {
			return errors.New("Enter an exact artifact media type")
		}
		if len(handle.Intents) == 0 || len(handle.Intents) > 2 {
			return errors.New("Choose View, Edit, or both")
		}
		for _, intent := range handle.Intents {
			if intent != "view" && intent != "edit" {
				return errors.New("Choose View, Edit, or both")
			}
		}
	}
	return nil
}

func (i *identity) workspaceApps(ctx context.Context, owner, admin string) ([]keycloakApp, error) {
	prefix := "subsoil-ws-" + owner + "-"
	query := url.Values{"clientId": {prefix}, "search": {"true"}, "max": {"100"}}
	var apps []keycloakApp
	if err := i.clients(ctx, admin, http.MethodGet, "?"+query.Encode(), nil, &apps); err != nil {
		return nil, err
	}
	return slices.DeleteFunc(apps, func(app keycloakApp) bool {
		return !strings.HasPrefix(app.ClientID, prefix) || app.Attributes["subsoil.workspace"] != owner
	}), nil
}

func (s *service) registeredApps(w http.ResponseWriter, r *http.Request) {
	token := authorization(w, r)
	if token == "" {
		return
	}
	if s.identity == nil {
		http.NotFound(w, r)
		return
	}
	owner, err := s.identity.owner(r.Context(), token)
	if errors.Is(err, noWorkspace) || (err == nil && owner != r.PathValue("workspace")) {
		writeJSON(w, http.StatusForbidden, &apiError{http.StatusForbidden, "WORKSPACE_DENIED", "Sign in to your verified workspace to manage apps"})
		return
	}
	if err != nil {
		s.workspaceError(w, err)
		return
	}

	if r.Method == http.MethodPost {
		var app appRegistration
		if decodeJSON(http.MaxBytesReader(w, r.Body, 16<<10), &app) != nil {
			writeJSON(w, http.StatusBadRequest, badRequest("Enter valid app details"))
			return
		}
		app.Name = strings.TrimSpace(app.Name)
		if err := app.validate(); err != nil {
			writeJSON(w, http.StatusBadRequest, badRequest(err.Error()))
			return
		}
		s.createWorkspaceApp(w, r, owner, app)
		return
	}

	admin, err := s.identity.adminToken(r.Context())
	if err != nil {
		s.workspaceError(w, err)
		return
	}
	clients, err := s.identity.workspaceApps(r.Context(), owner, admin)
	if err != nil {
		s.workspaceError(w, err)
		return
	}
	apps := []registeredApp{}
	for _, entry := range clients {
		if r.Method == http.MethodDelete {
			if entry.ClientID != r.PathValue("app") {
				continue
			}
			if err := s.identity.clients(r.Context(), admin, http.MethodDelete, "/"+url.PathEscape(entry.ID), nil, nil); err != nil {
				s.workspaceError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if app, ok := s.copyableApp(owner, entry); ok {
			apps = append(apps, app)
		}
	}
	if r.Method == http.MethodDelete {
		http.NotFound(w, r)
		return
	}
	slices.SortFunc(apps, func(a, b registeredApp) int { return strings.Compare(a.ID, b.ID) })
	writeJSON(w, http.StatusOK, apps)
}

func (s *service) copyableApp(owner string, entry keycloakApp) (registeredApp, bool) {
	var registration appRegistration
	if !entry.Enabled || json.Unmarshal([]byte(entry.Attributes["subsoil.app"]), &registration) != nil ||
		registration.validate() != nil {
		return registeredApp{}, false
	}
	return registeredApp{registration, entry.ClientID, entry.ClientID, s.config.AuthorizationServer,
		s.config.Registry, workspaceRepository(owner)}, true
}

func (s *service) createWorkspaceApp(w http.ResponseWriter, r *http.Request, owner string, registration appRegistration) {
	i := s.identity
	i.create.Lock()
	defer i.create.Unlock()
	admin, err := i.adminToken(r.Context())
	if err != nil {
		s.workspaceError(w, err)
		return
	}
	apps, err := i.workspaceApps(r.Context(), owner, admin)
	if err != nil {
		s.workspaceError(w, err)
		return
	}
	nameDigest := sha256.Sum256([]byte(strings.ToLower(registration.Name)))
	clientID := fmt.Sprintf("subsoil-ws-%s-%x", owner, nameDigest[:8])
	for _, entry := range apps {
		if entry.ClientID == clientID {
			writeJSON(w, http.StatusConflict, badRequest("An app with this name is already registered"))
			return
		}
	}
	if len(apps) >= workspaceAppLimit {
		writeJSON(w, http.StatusConflict, badRequest("Remove an app before registering another"))
		return
	}
	metadata, err := json.Marshal(registration)
	if err != nil || len(metadata) > 3500 {
		writeJSON(w, http.StatusBadRequest, badRequest("Shorten the app registration details"))
		return
	}
	app := keycloakApp{ClientID: clientID, Enabled: true, Attributes: map[string]string{
		"subsoil.workspace": owner, "subsoil.app": string(metadata),
		"pkce.code.challenge.method": "S256",
	}}
	err = i.clients(r.Context(), admin, http.MethodPost, "", map[string]any{
		"clientId": app.ClientID, "name": registration.Name, "protocol": "openid-connect",
		"enabled": true, "publicClient": true, "standardFlowEnabled": true, "consentRequired": true,
		"directAccessGrantsEnabled": false, "implicitFlowEnabled": false, "serviceAccountsEnabled": false,
		"fullScopeAllowed": false, "redirectUris": registration.RedirectURIs, "webOrigins": []string{developerOrigin},
		"defaultClientScopes": []string{}, "optionalClientScopes": []string{"workspace:read", "workspace:write"},
		"attributes": app.Attributes,
		"protocolMappers": []map[string]any{
			{
				"name": "Registry repository permissions", "protocol": "openid-connect", "protocolMapper": "subsoil-demo-registry-token",
				"config": map[string]string{"registry": i.registry, "service": "zot", "workspaceRepository": workspaceRepository(owner)},
			},
			{
				"name": "Account subject", "protocol": "openid-connect", "protocolMapper": "oidc-sub-mapper",
				"config": map[string]string{"access.token.claim": "true", "introspection.token.claim": "true"},
			},
		},
	}, nil)
	if errors.Is(err, errAppExists) {
		writeJSON(w, http.StatusConflict, badRequest("An app with this name is already registered"))
		return
	}
	if err != nil {
		s.workspaceError(w, err)
		return
	}
	copyable, _ := s.copyableApp(owner, app)
	writeJSON(w, http.StatusCreated, copyable)
}

func (s *service) workspace(w http.ResponseWriter, r *http.Request) {
	token := authorization(w, r)
	if token == "" {
		return
	}
	if s.identity == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	owner, err := s.identity.owner(r.Context(), token)
	if errors.Is(err, noWorkspace) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		s.workspaceError(w, err)
		return
	}
	repository := workspaceRepository(owner)
	if err := s.seedWorkspace(r.Context(), token, repository); err != nil {
		s.workspaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": owner, "repository": repository, "developer_origin": developerOrigin})
}

func (s *service) workspaceError(w http.ResponseWriter, err error) {
	// Log the upstream status and operation without returning credentials or tokens.
	log.Printf("workspace operation failed: %v", err)
	writeJSON(w, http.StatusBadGateway, &apiError{http.StatusBadGateway, "WORKSPACE_UNAVAILABLE", "Couldn’t load your workspace. Please try again."})
}
