package inboundmail

import (
	"encoding/base64"
	"strings"
	"testing"
)

const picture = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func wrapped(value string) string {
	var lines []string

	for len(value) > 20 {
		lines = append(lines, value[:20])
		value = value[20:]
	}

	return strings.Join(append(lines, value), "\r\n")
}

func TestAMessageGivesUpItsWordsAndItsFiles(t *testing.T) {
	raw := strings.Join([]string{
		"From: Rae Whitfield <rae@northwind.co>",
		"Subject: Export does nothing",
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="outer"`,
		"",
		"--outer",
		`Content-Type: multipart/alternative; boundary="inner"`,
		"",
		"--inner",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"The export button does nothing.",
		"--inner",
		"Content-Type: text/html; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"<p>The export button does =\r\nnothing.</p><img src=3D\"cid:shot@mail\">",
		"--inner--",
		"--outer",
		"Content-Type: image/png",
		"Content-Transfer-Encoding: base64",
		"Content-Id: <shot@mail>",
		"",
		wrapped(picture),
		"--outer",
		`Content-Type: application/pdf; name="report.pdf"`,
		"Content-Transfer-Encoding: base64",
		`Content-Disposition: attachment; filename="report.pdf"`,
		"",
		base64.StdEncoding.EncodeToString([]byte("a report")),
		"--outer--",
		"",
	}, "\r\n")

	held, err := readMessage([]byte(raw))
	if err != nil {
		t.Fatalf("reading the message failed: %v", err)
	}

	if !strings.Contains(held.text, "The export button does nothing.") {
		t.Fatalf("the plain words were lost: %q", held.text)
	}

	if !strings.Contains(held.html, `<img src="cid:shot@mail">`) {
		t.Fatalf(
			"the markup lost the picture: %q. The reference is what puts the file back where the "+
				"sender wrote it.",
			held.html,
		)
	}

	if len(held.attachments) != 2 {
		t.Fatalf("found %d files, want 2", len(held.attachments))
	}

	embedded, attached := held.attachments[0], held.attachments[1]

	if embedded.ContentID != "shot@mail" || !embedded.Embedded() {
		t.Fatalf("the picture is not tied to its place in the body: %+v", embedded)
	}

	decoded, err := base64.StdEncoding.DecodeString(picture)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if string(embedded.Content) != string(decoded) {
		t.Fatalf(
			"the picture came out %d bytes, want %d. A mail client wraps base64 at short lines, "+
				"and a decoder that refuses the wrapping stores a broken file.",
			len(embedded.Content), len(decoded),
		)
	}

	if attached.FileName != "report.pdf" || string(attached.Content) != "a report" {
		t.Fatalf("the attached file came out wrong: %+v", attached)
	}
}

func TestAFileWithNoNameStillGetsOne(t *testing.T) {
	raw := strings.Join([]string{
		"From: rae@northwind.co",
		"MIME-Version: 1.0",
		`Content-Type: multipart/related; boundary="edge"`,
		"",
		"--edge",
		"Content-Type: text/html",
		"",
		`<p>Look</p><img src="cid:pasted">`,
		"--edge",
		"Content-Type: image/png",
		"Content-Transfer-Encoding: base64",
		"Content-Id: <pasted>",
		"",
		picture,
		"--edge--",
		"",
	}, "\r\n")

	held, err := readMessage([]byte(raw))
	if err != nil {
		t.Fatalf("reading the message failed: %v", err)
	}

	if len(held.attachments) != 1 {
		t.Fatalf("found %d files, want 1", len(held.attachments))
	}

	if held.attachments[0].FileName != "attachment.png" {
		t.Fatalf(
			"the file is called %q. A picture pasted into a message arrives with no name, and a "+
				"file with none cannot be stored or listed.",
			held.attachments[0].FileName,
		)
	}
}

func TestAPlainMessageIsAllBodyAndNoFiles(t *testing.T) {
	raw := strings.Join([]string{
		"From: rae@northwind.co",
		"Subject: Nothing attached",
		"",
		"Just a sentence.",
		"",
	}, "\r\n")

	held, err := readMessage([]byte(raw))
	if err != nil {
		t.Fatalf("reading the message failed: %v", err)
	}

	if strings.TrimSpace(held.text) != "Just a sentence." {
		t.Fatalf("the body came out %q", held.text)
	}

	if len(held.attachments) != 0 {
		t.Fatalf("a message with nothing attached produced %d files", len(held.attachments))
	}
}
