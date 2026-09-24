package agentcapability

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	regularFileMode    = 0o644
	executableFileMode = 0o755
	executableBits     = 0o111
	macOSMetadataDir   = "__MACOSX"
)

func draftBundle(draft service.SkillDraft) (entity.AgentSkillBundle, error) {
	if len(draft.Archive) > 0 {
		return unzip(draft.Archive)
	}

	if strings.TrimSpace(draft.Instructions) == "" {
		return entity.AgentSkillBundle{}, entity.NewValidationError(
			entity.FieldError{Field: "instructions", Code: entity.ValidationCodeRequired},
		)
	}

	return entity.AgentSkillBundle{Files: []entity.AgentSkillFile{{
		Path:    entity.AgentSkillManifestFile,
		Content: []byte(draft.Instructions),
	}}}, nil
}

func archiveInvalid() error {
	return entity.NewValidationError(entity.FieldError{Field: "archive", Code: entity.ValidationCodeMalformed})
}

func archiveTooLarge() error {
	return entity.NewValidationError(entity.FieldError{Field: "archive", Code: entity.ValidationCodeTooLong})
}

func unzip(archive []byte) (entity.AgentSkillBundle, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return entity.AgentSkillBundle{}, archiveInvalid()
	}

	var (
		files []entity.AgentSkillFile
		total int64
	)

	for _, file := range reader.File {
		if file.FileInfo().IsDir() || strings.HasPrefix(file.Name, macOSMetadataDir+"/") {
			continue
		}

		if !file.Mode().IsRegular() {
			return entity.AgentSkillBundle{}, archiveInvalid()
		}

		if len(files) >= entity.AgentSkillBundleMaxFiles {
			return entity.AgentSkillBundle{}, archiveTooLarge()
		}

		opened, err := file.Open()
		if err != nil {
			return entity.AgentSkillBundle{}, archiveInvalid()
		}

		content, err := io.ReadAll(io.LimitReader(opened, entity.AgentSkillBundleMaxBytes-total+1))
		_ = opened.Close()

		if err != nil {
			return entity.AgentSkillBundle{}, archiveInvalid()
		}

		total += int64(len(content))
		if total > entity.AgentSkillBundleMaxBytes {
			return entity.AgentSkillBundle{}, archiveTooLarge()
		}

		files = append(files, entity.AgentSkillFile{
			Path:       file.Name,
			Executable: file.Mode().Perm()&executableBits != 0,
			Content:    content,
		})
	}

	return entity.AgentSkillBundle{Files: withoutCommonRoot(files)}, nil
}

func withoutCommonRoot(files []entity.AgentSkillFile) []entity.AgentSkillFile {
	for _, file := range files {
		if file.Path == entity.AgentSkillManifestFile {
			return files
		}
	}

	var root string

	for _, file := range files {
		if path.Base(file.Path) == entity.AgentSkillManifestFile && strings.Count(file.Path, "/") == 1 {
			root = path.Dir(file.Path) + "/"

			break
		}
	}

	if root == "" {
		return files
	}

	stripped := make([]entity.AgentSkillFile, 0, len(files))

	for _, file := range files {
		if !strings.HasPrefix(file.Path, root) {
			continue
		}

		file.Path = strings.TrimPrefix(file.Path, root)
		stripped = append(stripped, file)
	}

	return stripped
}

func pack(bundle entity.AgentSkillBundle) ([]byte, error) {
	var buffer bytes.Buffer

	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)

	for _, file := range bundle.Files {
		mode := int64(regularFileMode)
		if file.Executable {
			mode = executableFileMode
		}

		if err := archive.WriteHeader(&tar.Header{
			Name:     file.Path,
			Mode:     mode,
			Size:     int64(len(file.Content)),
			Typeflag: tar.TypeReg,
			Format:   tar.FormatPAX,
		}); err != nil {
			return nil, fmt.Errorf("write skill archive header: %w", err)
		}

		if _, err := archive.Write(file.Content); err != nil {
			return nil, fmt.Errorf("write skill archive file: %w", err)
		}
	}

	if err := archive.Close(); err != nil {
		return nil, fmt.Errorf("close skill archive: %w", err)
	}

	if err := compressed.Close(); err != nil {
		return nil, fmt.Errorf("compress skill archive: %w", err)
	}

	return buffer.Bytes(), nil
}
