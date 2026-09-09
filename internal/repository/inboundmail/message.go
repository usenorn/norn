package inboundmail

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"

	"github.com/usenorn/norn/internal/entity"
)

const nestingLimit = 8

type reading struct {
	text        string
	html        string
	attachments []entity.InboundAttachment
}

// readMessage takes a message apart as it was received. A mail client sends the same words
// twice over, as text and as markup, and puts a picture in the markup as a reference to a part
// of the message; only the original carries those parts, so this is where the files come from.
func readMessage(raw []byte) (reading, error) {
	message, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		return reading{}, fmt.Errorf("read the message: %w", err)
	}

	var held reading

	if err := readPart(&held, message.Header, message.Body, 0); err != nil {
		return reading{}, err
	}

	return held, nil
}

func readPart(held *reading, header mail.Header, body io.Reader, depth int) error {
	if depth > nestingLimit {
		return nil
	}

	contentType, parameters, err := mime.ParseMediaType(headerOf(header, "Content-Type", "text/plain"))
	if err != nil {
		contentType = "text/plain"
		parameters = map[string]string{}
	}

	if strings.HasPrefix(contentType, "multipart/") {
		boundary := parameters["boundary"]
		if boundary == "" {
			return nil
		}

		parts := multipart.NewReader(body, boundary)

		for {
			part, err := parts.NextPart()
			if errors.Is(err, io.EOF) {
				return nil
			}

			if err != nil {
				return fmt.Errorf("read a part of the message: %w", err)
			}

			if err := readPart(held, mail.Header(part.Header), part, depth+1); err != nil {
				_ = part.Close()

				return err
			}

			_ = part.Close()
		}
	}

	decoded, err := decode(body, headerOf(header, "Content-Transfer-Encoding", ""))
	if err != nil {
		return err
	}

	disposition, dispositionParameters, err := mime.ParseMediaType(
		headerOf(header, "Content-Disposition", "inline"),
	)
	if err != nil {
		disposition, dispositionParameters = "inline", map[string]string{}
	}

	name := fileName(parameters, dispositionParameters)
	contentID := strings.Trim(strings.TrimSpace(headerOf(header, "Content-Id", "")), "<>")

	// A part is the body when it is words nobody attached and nothing refers to; everything
	// else is a file, whether the sender attached it or wrote it into the message.
	if disposition != "attachment" && name == "" && contentID == "" {
		switch contentType {
		case "text/plain":
			held.text = join(held.text, string(decoded))

			return nil
		case "text/html":
			held.html = join(held.html, string(decoded))

			return nil
		}
	}

	if name == "" {
		name = entity.IntakeAttachmentFallback(contentType)
	}

	held.attachments = append(held.attachments, entity.InboundAttachment{
		FileName:    name,
		ContentType: contentType,
		ContentID:   contentID,
		Content:     decoded,
	})

	return nil
}

func decode(body io.Reader, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, whitespaceless(body)))
		if err != nil {
			return nil, fmt.Errorf("decode a part of the message: %w", err)
		}

		return decoded, nil
	case "quoted-printable":
		decoded, err := io.ReadAll(quotedprintable.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("decode a part of the message: %w", err)
		}

		return decoded, nil
	default:
		decoded, err := io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("read a part of the message: %w", err)
		}

		return decoded, nil
	}
}

// whitespaceless drops the line breaks base64 is wrapped at. The decoder refuses them, and a
// mail client wraps every attachment it sends.
func whitespaceless(body io.Reader) io.Reader {
	return &stripper{body: body}
}

type stripper struct {
	body io.Reader
}

func (s *stripper) Read(into []byte) (int, error) {
	read, err := s.body.Read(into)
	kept := 0

	for index := range read {
		switch into[index] {
		case '\r', '\n', ' ', '\t':
		default:
			into[kept] = into[index]
			kept++
		}
	}

	if kept == 0 && err == nil {
		return s.Read(into)
	}

	return kept, err
}

func fileName(parameters, dispositionParameters map[string]string) string {
	for _, given := range []string{dispositionParameters["filename"], parameters["name"]} {
		if decoded := decodeWord(given); decoded != "" {
			return decoded
		}
	}

	return ""
}

func decodeWord(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}

	decoded, err := new(mime.WordDecoder).DecodeHeader(trimmed)
	if err != nil {
		return trimmed
	}

	return decoded
}

func headerOf(header mail.Header, name, fallback string) string {
	if value := header.Get(name); value != "" {
		return value
	}

	return fallback
}

func join(held, addition string) string {
	if strings.TrimSpace(held) == "" {
		return addition
	}

	return held + "\n\n" + addition
}
