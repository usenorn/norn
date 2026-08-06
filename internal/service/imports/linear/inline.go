package linear

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/usenorn/norn/internal/entity"
)

const (
	uploadHost      = "uploads.linear.app"
	unnamedFileName = "image"
)

var inlineImage = regexp.MustCompile(`!\[([^\]]*)\]\(\s*<?([^)\s>]+)>?(?:\s+"[^"]*")?\s*\)`)

func upload(source string) bool {
	parsed, err := url.Parse(strings.TrimSpace(source))
	if err != nil {
		return false
	}

	return strings.EqualFold(parsed.Host, uploadHost)
}

// storedKey addresses an upload by its URL with the signature taken off. The same picture is
// read twice — once for the body that names it, once for the file record beside it — and
// Linear signs each read separately, so the query is the one part of the URL that cannot
// address anything: left on, the marker in the description and the record it points at would
// never be the same file.
func storedKey(source string) string {
	parsed, err := url.Parse(strings.TrimSpace(source))
	if err != nil {
		return strings.TrimSpace(source)
	}

	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed.String()
}

// marked replaces every inline upload with the marker the embed phase rewrites. A body left
// naming the source renders today and shows a hole the moment the signed URL expires. The
// marker names the upload and the body it sits in, because one image pasted into an issue and
// again into a comment on it is two rows to attach and only one address to tell them apart by.
func marked(body, owner string) string {
	return inlineImage.ReplaceAllStringFunc(body, func(match string) string {
		found := inlineImage.FindStringSubmatch(match)

		if !upload(found[2]) {
			return match
		}

		return "![" + found[1] + "](" + entity.ImportEmbedMarker(embedKey(found[2], owner)) + ")"
	})
}

// embedKey addresses one embedding rather than one file. The record it names carries which
// row the image belongs to, so two embeddings of the same upload cannot collapse onto a
// single staged row whose payload describes only whichever of them was written last.
func embedKey(source, owner string) string {
	return storedKey(source) + "#" + owner
}

func inlineUploads(body string) []string {
	sources := make([]string, 0)
	held := make(map[string]bool)

	for _, found := range inlineImage.FindAllStringSubmatch(body, -1) {
		if !upload(found[2]) {
			continue
		}

		key := storedKey(found[2])

		if held[key] {
			continue
		}

		held[key] = true

		sources = append(sources, found[2])
	}

	return sources
}

func fileNameOf(title, source string) string {
	if given := entity.AttachmentFileName(title); given != "" {
		return given
	}

	parsed, err := url.Parse(strings.TrimSpace(source))
	if err != nil {
		return unnamedFileName
	}

	base := entity.AttachmentFileName(parsed.Path)

	if unescaped, err := url.PathUnescape(base); err == nil {
		base = unescaped
	}

	if base == "" {
		return unnamedFileName
	}

	return base
}

// blobName keeps two files that arrived under the same name apart. entity.ImportBlobKey
// reduces whatever it is given to a last path segment, so the source's own identifier for
// the file has to travel inside the name rather than ahead of it.
func blobName(unique, name string) string {
	return entity.AttachmentFileName(unique) + "-" + entity.AttachmentFileName(name)
}

// uploadPath folds the whole path of an upload into one segment. Only the last segment of a
// Linear upload URL is the file name, and "image.png" is what it calls every pasted
// screenshot: naming the object after that alone puts every screenshot in the workspace at
// one key, and each import overwrites the last with different bytes and no report of it.
func uploadPath(source string) string {
	parsed, err := url.Parse(strings.TrimSpace(source))
	if err != nil {
		return entity.AttachmentFileName(source)
	}

	folded := strings.Trim(parsed.Path, "/")
	folded = strings.ReplaceAll(folded, "/", "-")

	if unescaped, err := url.PathUnescape(folded); err == nil {
		folded = unescaped
	}

	return entity.AttachmentFileName(folded)
}
