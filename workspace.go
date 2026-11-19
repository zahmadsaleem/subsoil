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

type capability struct {
	ArtifactType string   `json:"artifact_type"`
	Intents      []string `json:"intents"`
}

type fileCapability struct {
	MediaType string   `json:"media_type"`
	Intents   []string `json:"intents"`
}

type appRegistration struct {
	Name         string           `json:"name"`
	RedirectURIs []string         `json:"redirect_uris"`
	LaunchURL    string           `json:"launch_url,omitempty"`
	Handles      []capability     `json:"handles"`
	FileHandles  []fileCapability `json:"file_handles,omitempty"`
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
	return validateHandles(a.Handles, a.FileHandles)
}

func localAppURL(address string) bool {
	u, err := originURL(address, false)
	return err == nil && u.Scheme+"://"+u.Host == developerOrigin &&
		strings.HasPrefix(u.Path, "/") && !strings.ContainsAny(u.Path, "*\\")
}

func validateHandles(handles []capability, files []fileCapability) error {
	if count := len(handles) + len(files); count == 0 || count > 12 {
		return errors.New("Choose between one and twelve package or file types")
	}
	for _, handle := range handles {
		if err := validateCapability(handle.ArtifactType, handle.Intents); err != nil {
			return err
		}
	}
	for _, handle := range files {
		if err := validateCapability(handle.MediaType, handle.Intents); err != nil {
			return err
		}
	}
	return nil
}

func validateCapability(value string, intents []string) error {
	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil || len(params) != 0 || mediaType != value ||
		strings.Count(mediaType, "/") != 1 || strings.Contains(mediaType, "*") || len(mediaType) > 128 {
		return errors.New("Enter an exact media type such as application/pdf")
	}
	if len(intents) == 0 || len(intents) > 2 {
		return errors.New("Choose View, Edit, or both")
	}
	for _, intent := range intents {
		if !validIntent(intent) {
			return errors.New("Choose View, Edit, or both")
		}
	}
	return nil
}

func (i *identity) workspaceApps(ctx context.Context, owner, admin string) ([]keycloakApp, error) {
	prefix := "subsoil-ws-" + owner + "-"
	query := url.Values{"clientId": {prefix}, "search": {"true"}, "max": {"100"}}
	var apps []keycloakApp
	if err := i.adminRequest(ctx, admin, http.MethodGet, "/clients?"+query.Encode(), nil, &apps); err != nil {
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
		registered, failure := s.createWorkspaceApp(r.Context(), owner, app)
		if failure != nil {
			writeJSON(w, failure.status, failure)
			return
		}
		writeJSON(w, http.StatusCreated, registered)
		return
	}

	if r.Method == http.MethodDelete {
		if failure := s.removeWorkspaceApp(r.Context(), owner, r.PathValue("app")); failure != nil {
			if failure.status == http.StatusNotFound {
				http.NotFound(w, r)
			} else {
				writeJSON(w, failure.status, failure)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	apps, err := s.listWorkspaceApps(r.Context(), owner)
	if err != nil {
		s.workspaceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apps)
}

func (s *service) listWorkspaceApps(ctx context.Context, owner string) ([]registeredApp, error) {
	admin, err := s.identity.adminToken(ctx)
	if err != nil {
		return nil, err
	}
	clients, err := s.identity.workspaceApps(ctx, owner, admin)
	if err != nil {
		return nil, err
	}
	apps := []registeredApp{}
	for _, entry := range clients {
		if app, ok := s.copyableApp(owner, entry); ok {
			apps = append(apps, app)
		}
	}
	slices.SortFunc(apps, func(a, b registeredApp) int { return strings.Compare(a.ID, b.ID) })
	return apps, nil
}

func (s *service) removeWorkspaceApp(ctx context.Context, owner, appID string) *apiError {
	admin, err := s.identity.adminToken(ctx)
	if err != nil {
		return workspaceFailure(err)
	}
	clients, err := s.identity.workspaceApps(ctx, owner, admin)
	if err != nil {
		return workspaceFailure(err)
	}
	for _, entry := range clients {
		if entry.ClientID == appID {
			if err := s.identity.adminRequest(ctx, admin, http.MethodDelete, "/clients/"+url.PathEscape(entry.ID), nil, nil); err != nil {
				return workspaceFailure(err)
			}
			return nil
		}
	}
	return &apiError{http.StatusNotFound, "APP_NOT_FOUND", "This app is no longer registered"}
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

func (i *identity) appScopes(ctx context.Context, admin, repository string) ([]string, error) {
	scopes := []string{"workspace:read", "workspace:write"}
	for _, grant := range []struct{ action, label string }{
		{"pull", "Read files in your demo workspace"},
		{"push", "Publish versions in your demo workspace"},
	} {
		// The workspace owner determines the scope name, never the app submission.
		name := "repository:" + repository + ":" + grant.action
		err := i.adminRequest(ctx, admin, http.MethodPost, "/client-scopes", map[string]any{
			"name": name, "protocol": "openid-connect",
			"attributes": map[string]string{
				"include.in.token.scope":    "true",
				"display.on.consent.screen": "true",
				"consent.screen.text":       grant.label,
			},
		}, nil)
		if err != nil && !errors.Is(err, errIdentityConflict) {
			return nil, err
		}
		scopes = append(scopes, name)
	}
	return scopes, nil
}

func (s *service) createWorkspaceApp(ctx context.Context, owner string, registration appRegistration) (registeredApp, *apiError) {
	registration.Name = strings.TrimSpace(registration.Name)
	if err := registration.validate(); err != nil {
		return registeredApp{}, badRequest(err.Error())
	}
	if registration.Handles == nil {
		registration.Handles = []capability{}
	}
	i := s.identity
	i.create.Lock()
	defer i.create.Unlock()
	admin, err := i.adminToken(ctx)
	if err != nil {
		return registeredApp{}, workspaceFailure(err)
	}
	apps, err := i.workspaceApps(ctx, owner, admin)
	if err != nil {
		return registeredApp{}, workspaceFailure(err)
	}
	nameDigest := sha256.Sum256([]byte(strings.ToLower(registration.Name)))
	clientID := fmt.Sprintf("subsoil-ws-%s-%x", owner, nameDigest[:8])
	for _, entry := range apps {
		if entry.ClientID == clientID {
			return registeredApp{}, &apiError{http.StatusConflict, "INVALID_REQUEST", "An app with this name is already registered"}
		}
	}
	if len(apps) >= workspaceAppLimit {
		return registeredApp{}, &apiError{http.StatusConflict, "INVALID_REQUEST", "Remove an app before registering another"}
	}
	metadata, err := json.Marshal(registration)
	if err != nil || len(metadata) > 3500 {
		return registeredApp{}, badRequest("Shorten the app registration details")
	}
	repository := workspaceRepository(owner)
	scopes, err := i.appScopes(ctx, admin, repository)
	if err != nil {
		return registeredApp{}, workspaceFailure(err)
	}
	app := keycloakApp{ClientID: clientID, Enabled: true, Attributes: map[string]string{
		"subsoil.workspace": owner, "subsoil.app": string(metadata),
		"pkce.code.challenge.method": "S256",
	}}
	err = i.adminRequest(ctx, admin, http.MethodPost, "/clients", map[string]any{
		"clientId": app.ClientID, "name": registration.Name, "protocol": "openid-connect",
		"enabled": true, "publicClient": true, "standardFlowEnabled": true, "consentRequired": true,
		"directAccessGrantsEnabled": false, "implicitFlowEnabled": false, "serviceAccountsEnabled": false,
		"fullScopeAllowed": false, "redirectUris": registration.RedirectURIs, "webOrigins": []string{developerOrigin},
		"defaultClientScopes": []string{}, "optionalClientScopes": scopes,
		"attributes": app.Attributes,
		"protocolMappers": []map[string]any{
			{
				"name": "Registry repository permissions", "protocol": "openid-connect", "protocolMapper": "subsoil-demo-registry-token",
				"config": map[string]string{"registry": i.registry, "service": "zot", "workspaceRepository": repository},
			},
			{
				"name": "Account subject", "protocol": "openid-connect", "protocolMapper": "oidc-sub-mapper",
				"config": map[string]string{"access.token.claim": "true", "introspection.token.claim": "true"},
			},
		},
	}, nil)
	if errors.Is(err, errIdentityConflict) {
		return registeredApp{}, &apiError{http.StatusConflict, "INVALID_REQUEST", "An app with this name is already registered"}
	}
	if err != nil {
		return registeredApp{}, workspaceFailure(err)
	}
	copyable, _ := s.copyableApp(owner, app)
	return copyable, nil
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
	failure := workspaceFailure(err)
	writeJSON(w, failure.status, failure)
}

func workspaceFailure(err error) *apiError {
	// Log the upstream status and operation without returning credentials or tokens.
	log.Printf("workspace operation failed: %v", err)
	return &apiError{http.StatusBadGateway, "WORKSPACE_UNAVAILABLE", "Couldn’t load your workspace. Please try again."}
}
