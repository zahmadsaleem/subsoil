package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const emptyDescriptorType = "application/vnd.oci.empty.v1+json"

const (
	notesType     = "application/vnd.subsoil.demo.notes.v1"
	checklistType = "application/vnd.subsoil.demo.checklist.v1+json"
	reviewType    = "application/vnd.subsoil.demo.review.v1+json"
)

var workspaceSamples = []struct {
	tag, title, artifactType, mediaType, content string
}{
	{"notes-v1", "notes.txt", notesType, "text/plain", "Record a decision from your next project review.\n"},
	{"checklist-v1", "checklist.json", checklistType, "application/json", "{\n  \"items\": [\"Confirm the current drawing version\"]\n}\n"},
	{"review-v1", "review.json", reviewType, "application/json", "{\n  \"status\": \"For review\",\n  \"comment\": \"Confirm the proposed detail.\"\n}\n"},
}

type descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int               `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

func (s *service) seedWorkspace(ctx context.Context, token, repo string) error {
	for _, sample := range workspaceSamples {
		files := []sampleLayer{{sample.mediaType, []byte(sample.content), map[string]string{titleAnnotation: sample.title}}}
		if err := s.seedWorkspaceArtifact(ctx, token, repo, sample.tag, sample.artifactType, files, nil); err != nil {
			return err
		}
	}
	return nil
}

type sampleLayer struct {
	MediaType   string
	Bytes       []byte
	Annotations map[string]string
}

func (s *service) seedWorkspaceArtifact(ctx context.Context, token, repo, tag, artifactType string, files []sampleLayer, annotations map[string]string) error {
	base := "/v2/" + repo
	manifestPath := base + "/manifests/" + tag
	code, _, err := s.workspaceRequest(ctx, token, http.MethodHead, manifestPath, "", nil)
	if err != nil || code == http.StatusOK {
		return err
	}
	if code != http.StatusNotFound {
		return fmt.Errorf("check workspace sample returned %d", code)
	}
	config := []byte("{}")
	if err := s.seedBlob(ctx, token, base, config); err != nil {
		return err
	}
	layers := make([]descriptor, 0, len(files))
	for _, file := range files {
		if err := s.seedBlob(ctx, token, base, file.Bytes); err != nil {
			return err
		}
		layers = append(layers, descriptor{file.MediaType, digest(file.Bytes), len(file.Bytes), file.Annotations})
	}
	manifest := struct {
		SchemaVersion int               `json:"schemaVersion"`
		MediaType     string            `json:"mediaType"`
		ArtifactType  string            `json:"artifactType"`
		Config        descriptor        `json:"config"`
		Layers        []descriptor      `json:"layers"`
		Annotations   map[string]string `json:"annotations,omitempty"`
	}{2, manifestType, artifactType, descriptor{emptyDescriptorType, digest(config), len(config), nil}, layers, annotations}
	payload, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	code, _, err = s.workspaceRequest(ctx, token, http.MethodPut, manifestPath, manifestType, payload)
	if err != nil {
		return err
	}
	if code != http.StatusCreated {
		return fmt.Errorf("publish workspace sample returned %d", code)
	}
	return nil
}

func (s *service) seedBlob(ctx context.Context, token, repoPath string, body []byte) error {
	uploads := repoPath + "/blobs/uploads/"
	code, headers, err := s.workspaceRequest(ctx, token, http.MethodPost, uploads, "", nil)
	if err != nil {
		return err
	}
	if code != http.StatusAccepted {
		return fmt.Errorf("start workspace sample upload returned %d", code)
	}
	location, err := url.Parse(headers.Get("Location"))
	if err != nil || !strings.HasPrefix(location.EscapedPath(), uploads) {
		return fmt.Errorf("workspace sample upload returned an invalid location")
	}
	query := location.Query()
	query.Set("digest", digest(body))
	code, _, err = s.workspaceRequest(ctx, token, http.MethodPut,
		location.EscapedPath()+"?"+query.Encode(), "application/octet-stream", body)
	if err != nil {
		return err
	}
	if code != http.StatusCreated {
		return fmt.Errorf("upload workspace sample returned %d", code)
	}
	return nil
}

func (s *service) workspaceRequest(ctx context.Context, token, method, path, contentType string, data []byte) (int, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.upstream+path, bytes.NewReader(data))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Accept", manifestType)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	response, err := s.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	response.Body.Close()
	return response.StatusCode, response.Header, nil
}
