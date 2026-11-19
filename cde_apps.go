package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const cdeAppsPath = cdeWorkspacePath + "/apps"

type cdeAppSettings struct {
	Registrations []cdeAppRow
	Form          cdeAppForm
	Notice        string
}

type cdeAppRow struct {
	ID, Name, Config string
	Added            bool
}

type cdeAppForm struct {
	Name, Callbacks, LaunchURL, ViewPackages, EditPackages, ViewFiles, EditFiles string
}

func cdeTypeFields(values string) []string {
	return strings.FieldsFunc(values, func(char rune) bool {
		return char == ',' || char == '\n' || char == '\r'
	})
}

func cdeDeclaredTypes(view, edit string) (types []string, intents map[string][]string) {
	intents = make(map[string][]string)
	for _, field := range []struct{ values, intent string }{{view, "view"}, {edit, "edit"}} {
		for _, raw := range cdeTypeFields(field.values) {
			value := strings.TrimSpace(raw)
			if value == "" {
				continue
			}
			if len(intents[value]) == 0 {
				types = append(types, value)
			}
			if !slices.Contains(intents[value], field.intent) {
				intents[value] = append(intents[value], field.intent)
			}
		}
	}
	return types, intents
}

func (form cdeAppForm) registration() appRegistration {
	result := appRegistration{Name: form.Name, LaunchURL: strings.TrimSpace(form.LaunchURL)}
	for _, raw := range strings.Split(form.Callbacks, "\n") {
		if callback := strings.TrimSpace(raw); callback != "" {
			result.RedirectURIs = append(result.RedirectURIs, callback)
		}
	}
	packages, packageIntents := cdeDeclaredTypes(form.ViewPackages, form.EditPackages)
	for _, mediaType := range packages {
		result.Handles = append(result.Handles, capability{mediaType, packageIntents[mediaType]})
	}
	files, fileIntents := cdeDeclaredTypes(form.ViewFiles, form.EditFiles)
	for _, mediaType := range files {
		result.FileHandles = append(result.FileHandles, fileCapability{mediaType, fileIntents[mediaType]})
	}
	return result
}

func (c *cde) apps(w http.ResponseWriter, r *http.Request) {
	session, token := c.authorizePage(w, r, cdeAppsPath)
	if session == nil {
		return
	}
	page := c.page(session, cdeWorkspacePath)
	page.Title, page.Path = "Workspace apps", cdeAppsPath
	owner, status := c.workspaceOwner(r, token)
	if status != 0 {
		c.storeProblem(w, r, page, status)
		return
	}
	settings := &cdeAppSettings{
		Form: cdeAppForm{Callbacks: developerOrigin + "/", LaunchURL: developerOrigin + "/"},
	}
	page.AppSettings = settings
	status = http.StatusOK
	if r.Method == http.MethodPost {
		if !cdeValidForm(w, r, session) {
			return
		}
		if app := r.PathValue("app"); app != "" {
			if failure := c.s.removeWorkspaceApp(r.Context(), owner, app); failure != nil {
				status, settings.Notice = failure.status, failure.Message
			} else {
				http.Redirect(w, r, cdeAppsPath, http.StatusSeeOther)
				return
			}
		} else {
			fields := r.PostForm
			settings.Form = cdeAppForm{
				Name: fields.Get("name"), Callbacks: fields.Get("callbacks"), LaunchURL: fields.Get("launch_url"),
				ViewPackages: fields.Get("view_packages"), EditPackages: fields.Get("edit_packages"),
				ViewFiles: fields.Get("view_files"), EditFiles: fields.Get("edit_files"),
			}
			app, failure := c.s.createWorkspaceApp(r.Context(), owner, settings.Form.registration())
			if failure != nil {
				status, settings.Notice = failure.status, failure.Message
			} else {
				http.Redirect(w, r, cdeAppsPath+"?added="+url.QueryEscape(app.ID), http.StatusSeeOther)
				return
			}
		}
	}
	apps, err := c.s.listWorkspaceApps(r.Context(), owner)
	if err != nil {
		failure := workspaceFailure(err)
		status, page.Problem = failure.status, failure.Message
	}
	for _, app := range apps {
		config, _ := json.MarshalIndent(struct {
			registeredApp
			RedirectURI string `json:"redirect_uri"`
		}{app, app.RedirectURIs[0]}, "", "  ")
		settings.Registrations = append(settings.Registrations, cdeAppRow{
			ID: app.ID, Name: app.Name, Config: string(config), Added: r.URL.Query().Get("added") == app.ID,
		})
	}
	c.render(w, r, status, "page", page)
}

type cdeAppAction struct {
	Name, URL string
}

func cdeAppActions(route, fingerprint string, apps []application, manifest releaseManifest, layer *int) []cdeAppAction {
	matches, _ := matchingApps(apps, manifest, layer)
	var actions []cdeAppAction
	for _, app := range matches {
		for _, intent := range app.intents {
			query := url.Values{"version": {fingerprint}, "app": {app.ID}, "intent": {intent}}
			if layer != nil {
				query.Set("layer", strconv.Itoa(*layer))
			}
			action := "Open with " + app.Name
			if intent == "edit" {
				action = "Edit with " + app.Name
			}
			actions = append(actions, cdeAppAction{action, route + "/open?" + query.Encode()})
		}
	}
	return actions
}

func (c *cde) openApp(w http.ResponseWriter, r *http.Request) {
	route := strings.TrimSuffix(r.URL.Path, "/open")
	query := r.URL.Query()
	ref := query.Get("version")
	session, token := c.authorizePage(w, r, cdeVersionURL(route, ref))
	if session == nil {
		return
	}
	page := c.page(session, route)
	repo := cdeProjectRepo
	if page.Workspace {
		owner, status := c.workspaceOwner(r, token)
		if status != 0 {
			c.storeProblem(w, r, page, status)
			return
		}
		repo = workspaceRepository(owner)
	}
	layer, failure := parseLayer(query)
	if failure == nil {
		var target string
		target, failure = c.s.launchURL(r.Context(), token, launchRequest{
			ApplicationID: query.Get("app"), Repository: repo, Reference: ref, Intent: query.Get("intent"), Layer: layer,
		})
		if failure == nil {
			http.Redirect(w, r, target, http.StatusSeeOther)
			return
		}
	}
	page.Problem = failure.Message
	page.Path = cdeVersionURL(route, ref)
	c.render(w, r, failure.status, "page", page)
}
