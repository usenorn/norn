package entity

import (
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	SearchTextConfig     = "english"
	SearchPrefixConfig   = "simple"
	SearchQueryMaxLen    = 256
	SearchQueryMaxTokens = 12
	SearchTokenMaxLen    = 64
	SearchGroupMaxSize   = 25
	SearchPaletteSize    = 5
	SearchCandidateCap   = 500

	SearchSimilarityThreshold = "0.4"
)

var (
	ErrSearchQueryEmpty  = errors.New("search query has nothing to match on")
	ErrSearchKindUnknown = errors.New("that is not a kind of thing search returns")
)

type SearchKind string

const (
	SearchKindIssue   SearchKind = "issue"
	SearchKindComment SearchKind = "comment"
	SearchKindProject SearchKind = "project"
	SearchKindTeam    SearchKind = "team"
	SearchKindPerson  SearchKind = "person"
)

func SearchKinds() []SearchKind {
	return []SearchKind{
		SearchKindIssue,
		SearchKindComment,
		SearchKindProject,
		SearchKindTeam,
		SearchKindPerson,
	}
}

func (k SearchKind) Valid() bool {
	return slices.Contains(SearchKinds(), k)
}

type SearchQuery struct {
	Raw       string
	Stemmed   string
	Prefix    string
	Reference *IssueReference
}

func (q SearchQuery) Empty() bool {
	return q.Stemmed == "" && q.Prefix == "" && q.Reference == nil
}

func ParseSearchQuery(raw string) SearchQuery {
	trimmed := strings.TrimSpace(raw)

	if utf8.RuneCountInString(trimmed) > SearchQueryMaxLen {
		trimmed = string([]rune(trimmed)[:SearchQueryMaxLen])
	}

	query := SearchQuery{Raw: trimmed}

	if reference, err := ParseIssueReference(trimmed); err == nil {
		query.Reference = &reference
	}

	tokens := strings.Fields(strings.ReplaceAll(trimmed, `"`, " "))

	if len(tokens) > SearchQueryMaxTokens {
		tokens = tokens[:SearchQueryMaxTokens]
	}

	searchable := false

	for _, token := range tokens {
		if searchToken(token) != "" {
			searchable = true

			break
		}
	}

	if !searchable {
		return query
	}

	query.Stemmed = strings.Join(tokens, " ")
	query.Prefix = searchToken(tokens[len(tokens)-1])

	return query
}

func searchToken(raw string) string {
	var token strings.Builder

	for _, symbol := range raw {
		if unicode.IsLetter(symbol) || unicode.IsDigit(symbol) {
			token.WriteRune(symbol)
		}

		if token.Len() >= SearchTokenMaxLen {
			break
		}
	}

	return token.String()
}

type SearchResult struct {
	Kind      SearchKind
	ID        uuid.UUID
	IssueID   uuid.UUID
	Title     string
	Excerpt   string
	Reference string
	TeamKey   string
	Slug      string
	Status    string
	TitleHit  bool
	UpdatedAt time.Time
}

type SearchGroup struct {
	Kind    SearchKind
	Results []SearchResult
	More    bool
}

type SearchResults struct {
	Query  SearchQuery
	Groups []SearchGroup
	Fuzzy  bool
}

func (r SearchResults) Empty() bool {
	for _, group := range r.Groups {
		if len(group.Results) > 0 {
			return false
		}
	}

	return true
}
