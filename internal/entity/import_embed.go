package entity

import "strings"

const ImportEmbedScheme = "norn-import-attachment:"

func ImportEmbedMarker(externalID string) string {
	return ImportEmbedScheme + externalID
}

func ImportEmbedded(body string) bool {
	return strings.Contains(body, ImportEmbedScheme)
}
