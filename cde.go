package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	cdeProjectPath   = "/projects/riverside/files"
	cdeWorkspacePath = "/my-workspace"
	cdeProjectRepo   = "project/model"
)

var cdeTemplates = template.Must(template.ParseFS(static, "web/cde/pages.html"))

type cdeConfig struct {
	Origin         string `json:"origin"`
	IssuerUpstream string `json:"issuer_upstream"`
}

type cde struct {
	s              *service
	origin         string
	secure         bool
	issuerUpstream string
	sessions       cdeSessions
}

func newCDE(s *service, config cdeConfig) (*cde, error) {
	origin, err := originURL(config.Origin, false)
	if err != nil || origin.Path != "" {
		return nil, errors.New("set the public CDE origin without a path")
	}
	upstream, err := originURL(config.IssuerUpstream, true)
	public, _ := url.Parse(s.config.AuthorizationServer)
	if err != nil || upstream.Path == "" || upstream.EscapedPath() != public.EscapedPath() ||
		strings.HasSuffix(upstream.Path, "/") {
		return nil, errors.New("use the same realm path in the public and internal sign-in addresses")
	}
	return &cde{
		s: s, origin: config.Origin, secure: origin.Scheme == "https", issuerUpstream: config.IssuerUpstream,
		sessions: cdeSessions{pending: make(map[string]cdeLogin), sessions: make(map[string]*cdeSession)},
	}, nil
}

func randomCDEID() string {
	var bytes [32]byte
	rand.Read(bytes[:])
	return base64.RawURLEncoding.EncodeToString(bytes[:])
}

func (c *cde) routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, cdeProjectPath, http.StatusSeeOther)
	})
	mux.HandleFunc("GET /sign-in", c.web(c.signIn))
	mux.HandleFunc("GET /sign-in/start", c.web(c.startSignIn))
	mux.HandleFunc("GET "+cdeCallback, c.web(c.finishSignIn))
	mux.HandleFunc("POST /sign-out", c.web(c.signOut))
	mux.HandleFunc("GET "+cdeProjectPath, c.web(c.files))
	mux.HandleFunc("GET "+cdeProjectPath+"/download", c.web(c.download))
	mux.HandleFunc("GET "+cdeProjectPath+"/open", c.web(c.openApp))
	mux.HandleFunc("GET "+cdeWorkspacePath, c.web(c.files))
	mux.HandleFunc("POST "+cdeWorkspacePath+"/setup", c.web(c.setupWorkspace))
	mux.HandleFunc("POST "+cdeWorkspacePath+"/sample-package", c.web(c.addSamplePackage))
	mux.HandleFunc("GET "+cdeWorkspacePath+"/download", c.web(c.download))
	mux.HandleFunc("GET "+cdeWorkspacePath+"/open", c.web(c.openApp))
	mux.HandleFunc("GET "+cdeAppsPath, c.web(c.apps))
	mux.HandleFunc("POST "+cdeAppsPath, c.web(c.apps))
	mux.HandleFunc("POST "+cdeAppsPath+"/{app}/remove", c.web(c.apps))
	mux.HandleFunc("GET /cde/style.css", c.s.asset("web/cde/cde.css", "text/css; charset=utf-8"))
	mux.HandleFunc("GET /cde/actions.js", c.s.asset("web/cde/actions.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /cde/htmx-2.0.11.min.js", c.s.asset("web/cde/htmx-2.0.11.min.js", "text/javascript; charset=utf-8"))
}

func (c *cde) web(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		w.Header().Add("Vary", "HX-Request")
		handler(w, r)
	}
}

type cdeRevision struct {
	Name, URL string
	Selected  bool
}

type cdeFile struct {
	Name, Role, Size, URL string
	Actions               []cdeAppAction
}

type cdePage struct {
	Title, CSRF, Path, Query, Problem, Next         string
	Workspace, WorkspaceEnabled, CanSetUp, Filtered bool
	CanAddSample, PackageSelected                   bool
	Revisions                                       []cdeRevision
	PackageHistory                                  []cdeRelease
	LatestPackage                                   *cdeRelease
	Release                                         *cdeRelease
	Fingerprint, ShareURL, WorkspaceAppsURL         string
	PackageActions                                  []cdeAppAction
	AppsUnavailable                                 bool
	Files                                           []cdeFile
	AppSettings                                     *cdeAppSettings
}

type cdeRelease struct {
	Title, Publisher, URL string
	Published             time.Time
	FileCount             int
}

func (release cdeRelease) Date() string {
	if release.Published.IsZero() {
		return ""
	}
	return release.Published.UTC().Format("2 Jan 2006, 15:04 UTC")
}

func (release cdeRelease) Timestamp() string {
	return release.Published.UTC().Format(time.RFC3339)
}

type cdeLayer struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations"`
}

type releaseManifest struct {
	ArtifactType string            `json:"artifactType"`
	Layers       []cdeLayer        `json:"layers"`
	Annotations  map[string]string `json:"annotations"`
}

func cdeVersionURL(route, version string) string {
	return route + "?" + url.Values{"version": {version}}.Encode()
}

func (c *cde) render(w http.ResponseWriter, r *http.Request, status int, name string, page cdePage) {
	if name == "page" && r.Header.Get("HX-Request") == "true" {
		name = "work-area"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := cdeTemplates.ExecuteTemplate(w, name, page); err != nil {
		log.Printf("render CDE page: %v", err)
	}
}

func (c *cde) renderSignIn(w http.ResponseWriter, r *http.Request, status int, next, message string) {
	target := "/sign-in/start?" + url.Values{"next": {cdeDestination(next)}}.Encode()
	c.render(w, r, status, "sign-in", cdePage{Title: "Sign in", Next: target, Problem: message})
}

func (c *cde) authorizePage(w http.ResponseWriter, r *http.Request, destination string) (*cdeSession, string) {
	session := c.session(r)
	if session != nil {
		token, err := c.access(r.Context(), session)
		if err == nil {
			return session, token
		}
		if !errors.Is(err, errCDEReauthenticate) {
			page := c.page(session, strings.TrimSuffix(r.URL.Path, "/download"))
			page.Problem = "Sign-in is temporarily unavailable. Refresh the page to try again."
			c.render(w, r, http.StatusBadGateway, "page", page)
			return nil, ""
		}
		c.clearSession(w, r)
	}
	target := cdeSignInURL(destination)
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, target, http.StatusSeeOther)
	}
	return nil, ""
}

func (c *cde) page(session *cdeSession, route string) cdePage {
	page := cdePage{
		Title: "Project files", CSRF: session.csrf, Path: route,
		Workspace: route == cdeWorkspacePath, WorkspaceEnabled: c.s.identity != nil,
	}
	if page.Workspace {
		page.Title = "My workspace"
	}
	return page
}

func (c *cde) storeProblem(w http.ResponseWriter, r *http.Request, page cdePage, status int) {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		page.Problem = "You don’t have access to these files. Ask your project administrator for access."
		status = http.StatusForbidden
	case http.StatusNotFound:
		page.Problem = "That file or version couldn’t be found. Return to Files to see what’s available."
	case http.StatusBadRequest:
		page.Problem = "This version link is invalid. Choose a version from Files."
	default:
		page.Problem = "The file store is temporarily unavailable. Refresh the page to try again."
		status = http.StatusBadGateway
	}
	c.render(w, r, status, "page", page)
}

func (c *cde) workspaceOwner(r *http.Request, token string) (string, int) {
	if c.s.identity == nil {
		return "", http.StatusNotFound
	}
	owner, err := c.s.identity.owner(r.Context(), token)
	if errors.Is(err, noWorkspace) {
		return "", http.StatusForbidden
	}
	if err != nil {
		return "", http.StatusBadGateway
	}
	return owner, 0
}

func (manifest releaseManifest) release(route, fingerprint, fallbackTitle string) cdeRelease {
	title := manifest.Annotations[titleAnnotation]
	if title == "" {
		title = fallbackTitle
	}
	published, _ := time.Parse(time.RFC3339, manifest.Annotations[releasedAtAnnotation])
	return cdeRelease{
		Title: title, Publisher: manifest.Annotations[releasedByAnnotation], Published: published,
		URL: cdeVersionURL(route, fingerprint), FileCount: len(manifest.Layers),
	}
}

func (c *cde) files(w http.ResponseWriter, r *http.Request) {
	session, token := c.authorizePage(w, r, r.URL.RequestURI())
	if session == nil {
		return
	}
	page := c.page(session, r.URL.Path)
	repository := cdeProjectRepo
	if page.Workspace {
		owner, status := c.workspaceOwner(r, token)
		if status != 0 {
			c.storeProblem(w, r, page, status)
			return
		}
		repository = workspaceRepository(owner)
		page.WorkspaceAppsURL = cdeAppsPath
	}
	tags, failure := c.s.names(r.Context(), token, repository+"/tags/list", "tags")
	if page.Workspace && failure != nil && failure.status == http.StatusNotFound {
		page.CanSetUp = true
		c.render(w, r, http.StatusOK, "page", page)
		return
	}
	if failure != nil {
		c.storeProblem(w, r, page, failure.status)
		return
	}
	slices.Sort(tags)
	slices.Reverse(tags)
	page.Query = strings.TrimSpace(r.URL.Query().Get("q"))
	ref := r.URL.Query().Get("version")
	for _, tag := range slices.Compact(tags) {
		if !tagPattern.MatchString(tag) {
			continue
		}
		if samplePackageTag(tag) {
			fingerprint, manifest, failure := c.s.readRelease(r.Context(), token, repository, tag)
			if failure != nil {
				if failure.status == http.StatusNotFound {
					continue
				}
				c.storeProblem(w, r, page, failure.status)
				return
			}
			if manifest.ArtifactType == workPackageType {
				page.PackageHistory = append(page.PackageHistory, manifest.release(page.Path, fingerprint, "Work package"))
				continue
			}
		}
		if page.Workspace && strings.Contains(strings.ToLower(tag), strings.ToLower(page.Query)) {
			page.Revisions = append(page.Revisions, cdeRevision{Name: tag, URL: cdeVersionURL(page.Path, tag), Selected: ref == tag})
		}
	}
	if len(page.PackageHistory) > 0 {
		slices.SortStableFunc(page.PackageHistory, func(a, b cdeRelease) int {
			return b.Published.Compare(a.Published)
		})
		latest := page.PackageHistory[0]
		page.Filtered = !strings.Contains(strings.ToLower(latest.Title), strings.ToLower(page.Query))
		if !page.Filtered {
			page.LatestPackage = &latest
		}
	}
	page.CanAddSample = page.Workspace && len(page.PackageHistory) < 2
	if ref != "" {
		if status := c.selectedVersion(r, token, repository, ref, &page); status != 0 {
			c.storeProblem(w, r, page, status)
			return
		}
	}
	c.render(w, r, http.StatusOK, "page", page)
}

func (c *cde) selectedVersion(r *http.Request, token, repo, ref string, page *cdePage) int {
	fingerprint, manifest, failure := c.s.readRelease(r.Context(), token, repo, ref)
	if failure != nil {
		return failure.status
	}
	fallback := "Exact release"
	if !digestPattern.MatchString(ref) {
		fallback = ref
	}
	release := manifest.release(page.Path, fingerprint, fallback)
	page.Release = &release
	page.PackageSelected = manifest.ArtifactType == workPackageType
	page.Fingerprint = fingerprint
	page.ShareURL = c.origin + release.URL
	apps, failure := c.s.availableApps(r.Context(), token, repo)
	page.AppsUnavailable = failure != nil
	page.PackageActions = cdeAppActions(page.Path, fingerprint, apps, manifest, nil)
	for index, layer := range manifest.Layers {
		if !digestPattern.MatchString(layer.Digest) || layer.Size < 0 {
			return http.StatusBadGateway
		}
		query := url.Values{"version": {fingerprint}, "file": {strconv.Itoa(index)}}
		page.Files = append(page.Files, cdeFile{
			Name: layer.name(index), Role: layer.Annotations[fileRoleAnnotation], Size: cdeFileSize(layer.Size),
			URL: page.Path + "/download?" + query.Encode(), Actions: cdeAppActions(page.Path, fingerprint, apps, manifest, &index),
		})
	}
	return 0
}

func (layer cdeLayer) name(index int) string {
	name := path.Base(strings.ReplaceAll(layer.Annotations[titleAnnotation], "\\", "/"))
	if name == "." || name == "/" || name == ".." || strings.ContainsAny(name, "\r\n\x00") {
		return fmt.Sprintf("File %d", index+1)
	}
	return name
}

func cdeFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func (c *cde) download(w http.ResponseWriter, r *http.Request) {
	route := strings.TrimSuffix(r.URL.Path, "/download")
	ref := r.URL.Query().Get("version")
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
	index, err := strconv.Atoi(r.URL.Query().Get("file"))
	if err != nil || index < 0 || !digestPattern.MatchString(ref) {
		c.storeProblem(w, r, page, http.StatusBadRequest)
		return
	}
	_, manifest, failure := c.s.readRelease(r.Context(), token, repo, ref)
	if failure != nil {
		c.storeProblem(w, r, page, failure.status)
		return
	}
	if index >= len(manifest.Layers) {
		c.storeProblem(w, r, page, http.StatusNotFound)
		return
	}
	layer := manifest.Layers[index]
	if !digestPattern.MatchString(layer.Digest) || layer.Size < 0 {
		c.storeProblem(w, r, page, http.StatusBadGateway)
		return
	}
	file, _, failure := c.s.registryBytes(r.Context(), token, "/v2/"+repo+"/blobs/"+layer.Digest, min(layer.Size, 512<<20))
	if failure != nil {
		c.storeProblem(w, r, page, failure.status)
		return
	}
	if int64(len(file)) != layer.Size || digest(file) != layer.Digest {
		c.storeProblem(w, r, page, http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": layer.name(index)}))
	writeFile(w, "application/octet-stream", file)
}

func (c *cde) setupWorkspace(w http.ResponseWriter, r *http.Request) {
	session, token := c.authorizePage(w, r, cdeWorkspacePath)
	if session == nil || !cdeValidForm(w, r, session) {
		return
	}
	page := c.page(session, cdeWorkspacePath)
	owner, status := c.workspaceOwner(r, token)
	if status != 0 {
		c.storeProblem(w, r, page, status)
		return
	}
	if err := c.s.seedWorkspace(r.Context(), token, workspaceRepository(owner)); err != nil {
		c.storeProblem(w, r, page, http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, cdeWorkspacePath, http.StatusSeeOther)
}

func (c *cde) addSamplePackage(w http.ResponseWriter, r *http.Request) {
	session, token := c.authorizePage(w, r, cdeWorkspacePath)
	if session == nil || !cdeValidForm(w, r, session) {
		return
	}
	page := c.page(session, cdeWorkspacePath)
	owner, status := c.workspaceOwner(r, token)
	if status != 0 {
		c.storeProblem(w, r, page, status)
		return
	}
	if err := c.s.seedSamplePackage(r.Context(), token, workspaceRepository(owner)); err != nil {
		c.storeProblem(w, r, page, http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, cdeWorkspacePath, http.StatusSeeOther)
}
