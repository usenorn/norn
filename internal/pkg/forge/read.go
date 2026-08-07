package forge

import (
	"fmt"
	"io"
	"strings"
)

// readCapped refuses an oversized body rather than truncating it. A body cut off at a limit
// surfaces later as an unexpected end of input, at an offset that names neither the limit
// nor the call that hit it.
func readCapped(body io.Reader, limit int64) ([]byte, error) {
	read, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}

	if int64(len(read)) > limit {
		return nil, fmt.Errorf("%w (%d bytes)", ErrResponseTooLarge, limit)
	}

	return read, nil
}

func excerpt(body []byte) string {
	const most = 256

	trimmed := strings.TrimSpace(string(body))

	if len(trimmed) > most {
		return trimmed[:most]
	}

	return trimmed
}

// linkFor reads one relation out of an RFC 8288 Link header, which is how GitHub pages and
// how GitLab pages alongside its own page-number headers.
func linkFor(header, rel string) string {
	for _, part := range strings.Split(header, ",") {
		segments := strings.Split(strings.TrimSpace(part), ";")
		if len(segments) < 2 {
			continue
		}

		target := strings.TrimSpace(segments[0])
		if !strings.HasPrefix(target, "<") || !strings.HasSuffix(target, ">") {
			continue
		}

		for _, attribute := range segments[1:] {
			name, value, found := strings.Cut(strings.TrimSpace(attribute), "=")
			if !found || strings.TrimSpace(name) != "rel" {
				continue
			}

			if strings.Trim(strings.TrimSpace(value), `"`) == rel {
				return target[1 : len(target)-1]
			}
		}
	}

	return ""
}
