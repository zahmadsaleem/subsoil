package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	cdeLoginLifetime   = 5 * time.Minute
	cdeSessionLifetime = 8 * time.Hour
	cdeSessionCookie   = "subsoil_cde"
	cdeLoginCookie     = "subsoil_sign_in"
	cdeCallback        = "/sign-in/callback"
	keycloakOIDC       = "/protocol/openid-connect/"
)

var errCDEReauthenticate = errors.New("sign-in required")

type cdeLogin struct {
	state, verifier, destination string
	expires                      time.Time
}

type cdeTokens struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
	Seconds int64  `json:"expires_in"`
}

type cdeSession struct {
	mu      sync.Mutex
	tokens  cdeTokens
	renewAt time.Time
	expires time.Time
	csrf    string
}

type cdeSessions struct {
	mu       sync.Mutex
	pending  map[string]cdeLogin
	sessions map[string]*cdeSession
}

func (store *cdeSessions) prune(now time.Time) {
	for id, login := range store.pending {
		if !now.Before(login.expires) {
			delete(store.pending, id)
		}
	}
	for id, session := range store.sessions {
		if !now.Before(session.expires) {
			delete(store.sessions, id)
		}
	}
}

func (c *cde) tokenExchange(ctx context.Context, values url.Values) (cdeTokens, error) {
	values.Set("client_id", c.s.config.ClientID)
	values.Set("resource", c.s.config.Registry)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.issuerUpstream+keycloakOIDC+"token",
		strings.NewReader(values.Encode()))
	if err != nil {
		return cdeTokens{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c.secure {
		req.Header.Set("X-Forwarded-Proto", "https")
	}
	response, err := c.s.client.Do(req)
	if err != nil {
		return cdeTokens{}, err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusBadRequest, http.StatusUnauthorized:
		return cdeTokens{}, errCDEReauthenticate
	case http.StatusOK:
	default:
		return cdeTokens{}, fmt.Errorf("token exchange returned %d", response.StatusCode)
	}
	var tokens cdeTokens
	err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&tokens)
	if err != nil || tokens.Access == "" || tokens.Seconds < 1 || tokens.Seconds > int64((24*time.Hour)/time.Second) {
		return cdeTokens{}, errors.New("invalid sign-in response")
	}
	return tokens, nil
}

func (session *cdeSession) setTokens(tokens cdeTokens) {
	if tokens.Refresh == "" {
		tokens.Refresh = session.tokens.Refresh
	}
	session.tokens = tokens
	lifetime := time.Duration(tokens.Seconds) * time.Second
	session.renewAt = time.Now().Add(lifetime - min(lifetime/2, 30*time.Second))
}

func (c *cde) access(ctx context.Context, session *cdeSession) (string, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if time.Now().After(session.expires) {
		return "", errCDEReauthenticate
	}
	if time.Now().After(session.renewAt) {
		if session.tokens.Refresh == "" {
			return "", errCDEReauthenticate
		}
		tokens, err := c.tokenExchange(ctx, url.Values{
			"grant_type": {"refresh_token"}, "refresh_token": {session.tokens.Refresh},
		})
		if err != nil {
			return "", err
		}
		session.setTokens(tokens)
	}
	return "Bearer " + session.tokens.Access, nil
}

func (c *cde) cookieName(name string) string {
	if c.secure {
		return "__Host-" + name
	}
	return name
}

func (c *cde) cookie(w http.ResponseWriter, name, value string, age int) {
	http.SetCookie(w, &http.Cookie{
		Name: c.cookieName(name), Value: value, Path: "/", MaxAge: age,
		HttpOnly: true, Secure: c.secure, SameSite: http.SameSiteLaxMode,
	})
}

func (c *cde) cookieID(r *http.Request, name string) string {
	cookie, err := r.Cookie(c.cookieName(name))
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (c *cde) session(r *http.Request) *cdeSession {
	id := c.cookieID(r, cdeSessionCookie)
	c.sessions.mu.Lock()
	defer c.sessions.mu.Unlock()
	session := c.sessions.sessions[id]
	if session != nil && !time.Now().Before(session.expires) {
		delete(c.sessions.sessions, id)
		return nil
	}
	return session
}

func cdeDestination(value string) string {
	if len(value) > 1024 || !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "\\#") {
		return cdeProjectPath
	}
	u, err := url.ParseRequestURI(value)
	if err != nil || u.IsAbs() || u.Host != "" || u.RawPath != "" {
		return cdeProjectPath
	}
	switch u.Path {
	case cdeProjectPath, cdeWorkspacePath, cdeAppsPath:
		return u.RequestURI()
	default:
		return cdeProjectPath
	}
}

func cdeSignInURL(destination string) string {
	return "/sign-in?" + url.Values{"next": {cdeDestination(destination)}}.Encode()
}

func (c *cde) signIn(w http.ResponseWriter, r *http.Request) {
	if c.session(r) != nil {
		http.Redirect(w, r, cdeDestination(r.URL.Query().Get("next")), http.StatusSeeOther)
		return
	}
	c.renderSignIn(w, r, http.StatusOK, cdeDestination(r.URL.Query().Get("next")), "")
}

func (c *cde) startSignIn(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	login := cdeLogin{
		state: randomCDEID(), verifier: randomCDEID(),
		destination: cdeDestination(r.URL.Query().Get("next")), expires: now.Add(cdeLoginLifetime),
	}
	id := randomCDEID()
	c.sessions.mu.Lock()
	c.sessions.prune(now)
	delete(c.sessions.pending, c.cookieID(r, cdeLoginCookie))
	if len(c.sessions.pending) >= 2048 {
		c.sessions.mu.Unlock()
		c.renderSignIn(w, r, http.StatusServiceUnavailable, login.destination, "Sign-in is busy. Try again shortly.")
		return
	}
	c.sessions.pending[id] = login
	c.sessions.mu.Unlock()
	challenge := sha256.Sum256([]byte(login.verifier))
	query := url.Values{
		"client_id": {c.s.config.ClientID}, "redirect_uri": {c.origin + cdeCallback},
		"response_type": {"code"}, "scope": {strings.Join(c.s.browserScopes(), " ")},
		"resource": {c.s.config.Registry}, "state": {login.state},
		"code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])},
	}
	c.cookie(w, cdeLoginCookie, id, int(cdeLoginLifetime/time.Second))
	http.Redirect(w, r, c.s.config.AuthorizationServer+keycloakOIDC+"auth?"+query.Encode(), http.StatusSeeOther)
}

func (c *cde) finishSignIn(w http.ResponseWriter, r *http.Request) {
	id := c.cookieID(r, cdeLoginCookie)
	query := r.URL.Query()
	c.sessions.mu.Lock()
	login, present := c.sessions.pending[id]
	if present && len(query["state"]) == 1 && query.Get("state") == login.state {
		delete(c.sessions.pending, id)
	} else {
		present = false
	}
	c.sessions.mu.Unlock()
	if !present || !time.Now().Before(login.expires) {
		c.renderSignIn(w, r, http.StatusBadRequest, cdeProjectPath, "This sign-in link expired. Start sign-in again.")
		return
	}
	c.cookie(w, cdeLoginCookie, "", -1)
	if len(query["iss"]) != 1 || query.Get("iss") != c.s.config.AuthorizationServer ||
		len(query["code"]) != 1 || query.Get("code") == "" || query.Has("error") {
		c.renderSignIn(w, r, http.StatusBadRequest, login.destination, "Sign-in didn’t complete. Try again.")
		return
	}
	tokens, err := c.tokenExchange(r.Context(), url.Values{
		"grant_type": {"authorization_code"}, "redirect_uri": {c.origin + cdeCallback},
		"code": {query.Get("code")}, "code_verifier": {login.verifier},
	})
	if err != nil {
		c.renderSignIn(w, r, http.StatusBadGateway, login.destination, "Couldn’t complete sign-in. Try again.")
		return
	}
	sessionID := randomCDEID()
	now := time.Now()
	session := &cdeSession{expires: now.Add(cdeSessionLifetime), csrf: randomCDEID()}
	session.setTokens(tokens)
	c.sessions.mu.Lock()
	c.sessions.prune(now)
	delete(c.sessions.sessions, c.cookieID(r, cdeSessionCookie))
	if len(c.sessions.sessions) >= 4096 {
		c.sessions.mu.Unlock()
		c.renderSignIn(w, r, http.StatusServiceUnavailable, login.destination, "Sign-in is busy. Try again shortly.")
		return
	}
	c.sessions.sessions[sessionID] = session
	c.sessions.mu.Unlock()
	c.cookie(w, cdeSessionCookie, sessionID, int(cdeSessionLifetime/time.Second))
	http.Redirect(w, r, login.destination, http.StatusSeeOther)
}

func (c *cde) signOut(w http.ResponseWriter, r *http.Request) {
	session := c.session(r)
	if !cdeValidForm(w, r, session) {
		return
	}
	c.clearSession(w, r)
	http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
}

func (c *cde) clearSession(w http.ResponseWriter, r *http.Request) {
	c.sessions.mu.Lock()
	delete(c.sessions.sessions, c.cookieID(r, cdeSessionCookie))
	c.sessions.mu.Unlock()
	c.cookie(w, cdeSessionCookie, "", -1)
}

func cdeValidForm(w http.ResponseWriter, r *http.Request, session *cdeSession) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if session == nil || r.ParseForm() != nil || r.PostForm.Get("csrf") != session.csrf {
		http.Error(w, "This action expired. Refresh the page and try again.", http.StatusForbidden)
		return false
	}
	return true
}
