package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const manifestType = "application/vnd.oci.image.manifest.v1+json"

var (
	repositoryPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*(/[a-z0-9][a-z0-9._-]*)*$`)
	tagPattern        = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$`)
	digestPattern     = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
)

func validRepository(repo string) bool {
	return len(repo) <= 255 && repositoryPattern.MatchString(repo)
}

func digest(body []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256(body))
}

func (s *service) get(ctx context.Context, token, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.upstream+path, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	req.Header.Set("Accept", manifestType)
	return s.client.Do(req)
}

func (s *service) registryBytes(ctx context.Context, token, path string, limit int64) ([]byte, http.Header, *apiError) {
	res, err := s.get(ctx, token, path)
	if err != nil {
		return nil, nil, unavailable()
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		status := res.StatusCode
		switch status {
		case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		default:
			status = http.StatusBadGateway
		}
		return nil, nil, &apiError{status, "FILE_STORE_REQUEST_FAILED", "These files are unavailable. Sign in again or ask the owner for access."}
	}
	if res.ContentLength > limit {
		return nil, nil, unavailable()
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil || int64(len(body)) > limit {
		return nil, nil, unavailable()
	}
	return body, res.Header, nil
}

func (s *service) sourceManifest(ctx context.Context, token, repo, ref string) (string, []byte, *apiError) {
	if !validRepository(repo) || (!tagPattern.MatchString(ref) && !digestPattern.MatchString(ref)) {
		return "", nil, badRequest("Choose a valid folder and version")
	}
	body, _, failure := s.registryBytes(ctx, token, "/v2/"+repo+"/manifests/"+ref, 1<<20)
	if failure != nil {
		return "", nil, failure
	}
	fingerprint := digest(body)
	if digestPattern.MatchString(ref) && ref != fingerprint {
		return "", nil, badRequest("The file store returned another version")
	}
	return fingerprint, body, nil
}

func (s *service) readRelease(ctx context.Context, token, repo, ref string) (string, releaseManifest, *apiError) {
	fingerprint, body, failure := s.sourceManifest(ctx, token, repo, ref)
	var manifest releaseManifest
	if failure == nil && json.Unmarshal(body, &manifest) != nil {
		failure = unavailable()
	}
	return fingerprint, manifest, failure
}

func (s *service) ready(w http.ResponseWriter, r *http.Request) {
	res, err := s.get(r.Context(), "", "/v2/")
	if err == nil {
		res.Body.Close()
		if (res.StatusCode >= 200 && res.StatusCode < 300) || res.StatusCode == http.StatusUnauthorized {
			w.WriteHeader(http.StatusOK)
			return
		}
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}

func (s *service) catalog(w http.ResponseWriter, r *http.Request) {
	token := authorization(w, r)
	if token == "" {
		return
	}
	s.list(w, r, token, "_catalog", "repositories")
}

func (s *service) file(w http.ResponseWriter, r *http.Request) {
	token := authorization(w, r)
	if token == "" {
		return
	}
	path := r.PathValue("path")
	if repo, ok := strings.CutSuffix(path, "/tags/list"); ok {
		if !validRepository(repo) {
			failure := badRequest("Choose a valid folder")
			writeJSON(w, failure.status, failure)
			return
		}
		s.list(w, r, token, path, "tags")
		return
	}

	if i := strings.LastIndex(path, "/manifests/"); i >= 0 {
		fingerprint, bytes, failure := s.sourceManifest(r.Context(), token, path[:i], path[i+len("/manifests/"):])
		if failure != nil {
			writeJSON(w, failure.status, failure)
			return
		}
		w.Header().Set("Docker-Content-Digest", fingerprint)
		writeFile(w, manifestType, bytes)
		return
	}

	if i := strings.LastIndex(path, "/blobs/"); i >= 0 {
		repo, fingerprint := path[:i], path[i+len("/blobs/"):]
		if validRepository(repo) && digestPattern.MatchString(fingerprint) {
			body, _, failure := s.registryBytes(r.Context(), token, "/v2/"+path, 512<<20)
			if failure != nil {
				writeJSON(w, failure.status, failure)
				return
			}
			if digest(body) != fingerprint {
				failure = badRequest("The downloaded file failed its integrity check")
				writeJSON(w, failure.status, failure)
				return
			}
			writeFile(w, "application/octet-stream", body)
			return
		}
	}
	failure := badRequest("Choose a folder, version or file")
	writeJSON(w, failure.status, failure)
}

func writeFile(w http.ResponseWriter, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(body)
}

func (s *service) list(w http.ResponseWriter, r *http.Request, token, path, field string) {
	query := r.URL.Query()
	limit := 20
	if query.Has("n") {
		n, err := strconv.Atoi(query.Get("n"))
		if err != nil || n < 1 || n > 1000 {
			failure := badRequest("Choose a page size between 1 and 1000")
			writeJSON(w, failure.status, failure)
			return
		}
		limit = n
	}
	names, failure := s.names(r.Context(), token, path, field)
	if failure != nil {
		writeJSON(w, failure.status, failure)
		return
	}

	last, filter := query.Get("last"), strings.ToLower(query.Get("q"))
	slices.Sort(names)
	names = slices.Compact(names)
	names = slices.DeleteFunc(names, func(name string) bool {
		return name <= last || !strings.Contains(strings.ToLower(name), filter)
	})
	if len(names) > limit {
		names = names[:limit]
		next := url.Values{"n": {strconv.Itoa(limit)}, "last": {names[len(names)-1]}, "q": {filter}}
		w.Header().Set("Link", fmt.Sprintf("</files/%s?%s>; rel=\"next\"", path, next.Encode()))
	}
	writeJSON(w, http.StatusOK, map[string][]string{field: names})
}

func (s *service) names(ctx context.Context, token, path, field string) ([]string, *apiError) {
	names := []string{}
	last := ""
	for range 1000 {
		query := url.Values{"n": {"100"}, "last": {last}}
		body, headers, failure := s.registryBytes(ctx, token, "/v2/"+path+"?"+query.Encode(), 1<<20)
		if failure != nil {
			return nil, failure
		}
		var listing map[string]json.RawMessage
		if json.Unmarshal(body, &listing) != nil {
			return nil, unavailable()
		}
		var items []string
		if json.Unmarshal(listing[field], &items) != nil {
			return nil, unavailable()
		}
		names = append(names, items...)
		if headers.Get("Link") == "" {
			return names, nil
		}
		if len(items) == 0 || items[len(items)-1] <= last {
			return nil, unavailable()
		}
		last = items[len(items)-1]
	}
	return nil, unavailable()
}
