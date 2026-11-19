package main

import (
	"context"
	"encoding/json"
	"strings"
)

const (
	workPackageType      = "application/vnd.subsoil.demo.work-package.v1"
	titleAnnotation      = "org.opencontainers.image.title"
	fileRoleAnnotation   = "dev.subsoil.file.role"
	releasedByAnnotation = "dev.subsoil.release.by"
	releasedAtAnnotation = "dev.subsoil.release.at"
)

func samplePackageTag(tag string) bool {
	number, ok := strings.CutPrefix(tag, "work-package-")
	return ok && len(number) == 2 && number[0] >= '0' && number[0] <= '9' && number[1] >= '0' && number[1] <= '9'
}

func (s *service) seedSamplePackage(ctx context.Context, token, repo string) error {
	body, err := static.ReadFile("demo/work-package/releases.json")
	if err != nil {
		return err
	}
	var sample struct {
		Title        string `json:"title"`
		ArtifactType string `json:"artifact_type"`
		Releases     []struct {
			Tag        string `json:"tag"`
			ReleasedBy string `json:"released_by"`
			ReleasedAt string `json:"released_at"`
			Files      []struct {
				Source, Name, Role string
				MediaType          string `json:"media_type"`
			}
		}
	}
	if err := json.Unmarshal(body, &sample); err != nil {
		return err
	}
	for _, release := range sample.Releases {
		var files []sampleLayer
		for _, file := range release.Files {
			content, err := static.ReadFile("demo/" + file.Source)
			if err != nil {
				return err
			}
			files = append(files, sampleLayer{file.MediaType, content,
				map[string]string{titleAnnotation: file.Name, fileRoleAnnotation: file.Role}})
		}
		if err := s.seedWorkspaceArtifact(ctx, token, repo, release.Tag, sample.ArtifactType, files, map[string]string{
			titleAnnotation: sample.Title, releasedByAnnotation: release.ReleasedBy, releasedAtAnnotation: release.ReleasedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}
