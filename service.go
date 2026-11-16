package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const handoffProtocol = "oci-handoff/1"

//go:embed web/* assets/brand/subsoil-logo.svg assets/brand/subsoil-symbol.svg
var static embed.FS

type application struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	LaunchURL string       `json:"launch_url"`
	Handles   []capability `json:"handles"`
}

type configuration struct {
	Registry            string        `json:"registry"`
	RegistryUpstream    string        `json:"registry_upstream"`
	AuthorizationServer string        `json:"authorization_server"`
	ClientID            string        `json:"client_id"`
	Scopes              []string      `json:"scopes"`
	Applications        []application `json:"applications"`
}

type service struct {
	config       configuration
	upstream     string
	issuerOrigin string
	client       *http.Client
	identity     *identity
}

var hostname = regexp.MustCompile(`(?i)^[a-z0-9][a-z0-9.-]*$`)

func originURL(value string, allowPrivateHTTP bool) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil || strings.ContainsAny(value, "\\#") || strings.ContainsFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) {
		return nil, errors.New("invalid URL")
	}
	host := u.Hostname()
	ip, ipErr := netip.ParseAddr(host)
	if (!hostname.MatchString(host) && (ipErr != nil || ip.Zone() != "")) ||
		u.User != nil || u.ForceQuery || u.RawQuery != "" ||
		(u.Scheme != "https" && !(u.Scheme == "http" && (allowPrivateHTTP || host == "localhost" || ip.IsLoopback()))) {
		return nil, errors.New("use an HTTPS URL or a permitted local HTTP URL without credentials, query or fragment")
	}
	port := u.Port()
	if port != "" {
		number, err := strconv.ParseUint(port, 10, 16)
		if err != nil || number == 0 || strconv.FormatUint(number, 10) != port {
			return nil, errors.New("use a valid URL port")
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return nil, errors.New("use a valid URL port")
	}
	return u, nil
}

func newService(config configuration) (*service, error) {
	public, err := originURL(config.Registry, false)
	if err != nil || config.Registry != public.Scheme+"://"+strings.ToLower(public.Host) ||
		(public.Scheme == "https" && public.Port() == "443") ||
		(public.Scheme == "http" && public.Port() == "80") {
		return nil, errors.New("use a registry origin without a trailing slash")
	}
	upstream, err := originURL(config.RegistryUpstream, true)
	if err != nil || (upstream.Path != "" && upstream.EscapedPath() != "/") {
		return nil, errors.New("configure an exact registry upstream origin")
	}
	issuer, err := originURL(config.AuthorizationServer, false)
	if err != nil {
		return nil, fmt.Errorf("authorization server: %w", err)
	}
	if strings.TrimSpace(config.ClientID) == "" || len(config.Scopes) == 0 {
		return nil, errors.New("configure the browser's OAuth client and scopes")
	}

	ids := make(map[string]bool)
	for _, app := range config.Applications {
		if strings.TrimSpace(app.ID) == "" || ids[app.ID] || strings.TrimSpace(app.Name) == "" {
			return nil, errors.New("choose unique application IDs and names")
		}
		ids[app.ID] = true
		if _, err := originURL(app.LaunchURL, false); err != nil {
			return nil, fmt.Errorf("application %q: %w", app.ID, err)
		}
		if err := validateHandles(app.Handles); err != nil {
			return nil, fmt.Errorf("application %q: %w", app.ID, err)
		}
	}

	s := &service{
		config:       config,
		upstream:     upstream.Scheme + "://" + upstream.Host,
		issuerOrigin: issuer.Scheme + "://" + issuer.Host,
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	if file := os.Getenv("SUBSOIL_REGISTRAR_FILE"); file != "" {
		s.identity, err = loadIdentity(file, s.client, config)
		if err != nil {
			return nil, fmt.Errorf("load workspace registrar: %w", err)
		}
	}
	return s, nil
}

func (s *service) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/explore", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("GET /workflow/config", s.browserConfig)
	mux.HandleFunc("POST /workspace", s.workspace)
	mux.HandleFunc("GET /workspaces/{workspace}/apps", s.registeredApps)
	mux.HandleFunc("POST /workspaces/{workspace}/apps", s.registeredApps)
	mux.HandleFunc("DELETE /workspaces/{workspace}/apps/{app}", s.registeredApps)
	mux.HandleFunc("GET /files/_catalog", s.catalog)
	mux.HandleFunc("GET /files/{path...}", s.file)
	mux.HandleFunc("GET /handoff/applications", s.applications)
	mux.HandleFunc("POST /handoff/launches", s.launch)
	mux.HandleFunc("GET /explore", s.asset("web/explorer.html", "text/html; charset=utf-8"))
	mux.HandleFunc("GET /explore/app.js", s.asset("web/explorer.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /explore/workspace.js", s.asset("web/workspace.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /explore/style.css", s.asset("web/explorer.css", "text/css; charset=utf-8"))
	mux.HandleFunc("GET /handoff/controls.js", s.asset("web/handoff_ui.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /handoff/controls.css", s.asset("web/handoff_ui.css", "text/css; charset=utf-8"))
	mux.HandleFunc("GET /brand/logo.svg", s.asset("assets/brand/subsoil-logo.svg", "image/svg+xml"))
	mux.HandleFunc("GET /brand/symbol.svg", s.asset("assets/brand/subsoil-symbol.svg", "image/svg+xml"))
	return mux
}

func (s *service) asset(path, contentType string) http.HandlerFunc {
	body, err := static.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self' "+s.issuerOrigin+"; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		w.Write(body)
	}
}

func (s *service) browserConfig(w http.ResponseWriter, _ *http.Request) {
	scopes := s.config.Scopes
	if s.identity != nil {
		scopes = append(append([]string{}, scopes...), "openid", "workspace:read", "workspace:write")
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"registry": s.config.Registry, "handoff_protocol": handoffProtocol,
		"authorization_server": s.config.AuthorizationServer, "client_id": s.config.ClientID,
		"scopes": scopes, "workspaces": s.identity != nil,
	})
}

type apiError struct {
	status  int
	Code    string `json:"error"`
	Message string `json:"message"`
}

func badRequest(message string) *apiError {
	return &apiError{http.StatusBadRequest, "INVALID_REQUEST", message}
}

func unavailable() *apiError {
	return &apiError{http.StatusBadGateway, "FILE_STORE_UNAVAILABLE", "Could not read these files. Refresh or ask the owner for access."}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func authorization(w http.ResponseWriter, r *http.Request) string {
	token := r.Header.Get("Authorization")
	if !strings.HasPrefix(token, "Bearer ") || strings.TrimSpace(token[7:]) == "" {
		writeJSON(w, http.StatusUnauthorized, &apiError{http.StatusUnauthorized, "SIGN_IN_REQUIRED", "Sign in to browse your files"})
		return ""
	}
	return token
}

func decodeJSON(r io.Reader, value any) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return errors.New("expected one JSON value")
	}
	return nil
}
