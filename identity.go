package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
)

const developerOrigin = "http://localhost:5173"

var workspaceID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var noWorkspace = errors.New("no verified workspace")
var errIdentityConflict = errors.New("identity record already exists")

type identity struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Issuer       string `json:"issuer"`
	admin        string
	client       *http.Client
	registry     string
	browser      string
	create       sync.Mutex
}

func loadIdentity(file string, client *http.Client, config configuration) (*identity, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	service := &identity{client: client, registry: config.Registry, browser: config.ClientID}
	if err := decodeJSON(bytes.NewReader(data), service); err != nil {
		return nil, errors.New("invalid registrar credentials")
	}
	issuer, err := originURL(service.Issuer, true)
	if err != nil || !strings.HasPrefix(issuer.Path, "/realms/") ||
		strings.Count(issuer.Path, "/") != 2 || service.ClientID == "" || service.ClientSecret == "" {
		return nil, errors.New("invalid registrar issuer or credentials")
	}
	service.admin = issuer.Scheme + "://" + issuer.Host + "/admin" + issuer.EscapedPath()
	return service, nil
}

func (i *identity) form(ctx context.Context, endpoint string, form url.Values, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		i.Issuer+"/protocol/openid-connect/"+endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(i.ClientID, i.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return i.send(req, result)
}

func (i *identity) send(req *http.Request, result any) error {
	req.Header.Set("X-Forwarded-Proto", "https")
	response, err := i.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusConflict {
		return errIdentityConflict
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("identity service returned %d", response.StatusCode)
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(result)
	}
	return nil
}

func (i *identity) owner(ctx context.Context, authorization string) (string, error) {
	var result struct {
		Active   bool   `json:"active"`
		Subject  string `json:"sub"`
		ClientID string `json:"client_id"`
		Scope    string `json:"scope"`
	}
	err := i.form(ctx, "token/introspect", url.Values{
		"token": {strings.TrimPrefix(authorization, "Bearer ")},
	}, &result)
	if err != nil {
		return "", err
	}
	if !result.Active || result.ClientID != i.browser || !workspaceID.MatchString(result.Subject) ||
		!slices.Contains(strings.Fields(result.Scope), "workspace:write") {
		return "", noWorkspace
	}
	return result.Subject, nil
}

func (i *identity) adminToken(ctx context.Context) (string, error) {
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := i.form(ctx, "token", url.Values{"grant_type": {"client_credentials"}}, &result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", errors.New("identity service returned no registrar token")
	}
	return result.AccessToken, nil
}

func (i *identity) adminRequest(ctx context.Context, token, method, path string, body, result any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, i.admin+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return i.send(req, result)
}
