package csvfile

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/usenorn/norn/internal/entity"
	"github.com/usenorn/norn/internal/service"
)

const (
	noteTeamStandsIn = "A file of rows names no team. One team stands in for the whole file and " +
		"has to be mapped onto a team in this workspace before the run: an issue with nowhere to " +
		"land is an issue that is skipped."
	noteHeaderRead = "The first row reads as the names of the columns, so it is what the mapping " +
		"below is proposed from and it is not imported as an issue."
	noteHeaderMissing = "The first row does not read as the names of the columns, so every row is " +
		"imported and the columns have to be chosen by position."
	noteEmptyFile = "This file holds no rows at all."
)

// Probe reads the head of the file and answers with what the columns look like. It stages
// nothing: a run cannot be told which column is a title until somebody has seen the names,
// and this is the one call a source is asked for outside the staging job.
func (s *Source) Probe(
	ctx context.Context,
	held service.ImportSourceConfig,
) (entity.ImportCatalogue, error) {
	settings, err := s.settings(entity.ImportIssue, held)
	if err != nil {
		return entity.ImportCatalogue{}, err
	}

	file, err := s.open(ctx, entity.ImportIssue, settings)
	if err != nil {
		return entity.ImportCatalogue{}, err
	}

	defer func() { _ = file.Close() }()

	head := buffered(file)

	if _, err := preamble(head, entity.ImportIssue); err != nil {
		return entity.ImportCatalogue{}, err
	}

	comma, guessed, err := separator(head, settings, entity.ImportIssue)
	if err != nil {
		return entity.ImportCatalogue{}, err
	}

	reader := rowReader(io.LimitReader(head, probeWindow), comma)

	rows, whole, err := firstRows(reader)
	if err != nil {
		return entity.ImportCatalogue{}, err
	}

	if len(rows) == 0 {
		return entity.ImportCatalogue{
			Notes: []string{delimiterNote(comma, guessed), noteEmptyFile, noteTeamStandsIn},
		}, nil
	}

	names := headerRow(settings, rows[0])
	visible := len(rows)

	if names {
		visible--
	}

	return entity.ImportCatalogue{
		Columns: proposals(rows[0], names),
		Notes: []string{
			delimiterNote(comma, guessed),
			headerNote(names),
			rowsNote(visible, whole),
			noteTeamStandsIn,
		},
	}, nil
}

func firstRows(reader *csv.Reader) ([][]string, bool, error) {
	rows := make([][]string, 0)

	for {
		fields, err := reader.Read()
		if err != nil {
			if _, malformed := brokenRow(err); !malformed && !errors.Is(err, io.EOF) {
				return nil, false, unreadable(entity.ImportIssue, err)
			}

			return rows, reader.InputOffset() < probeWindow, nil
		}

		rows = append(rows, fields)
	}
}

func proposals(first []string, names bool) []entity.ImportColumn {
	columns := make([]entity.ImportColumn, 0, len(first))
	claimed := make(map[string]bool, len(first))

	for index, cell := range first {
		column := entity.ImportColumn{
			Index:      index,
			Proposed:   targetIgnore,
			Confidence: confidenceNone,
		}

		if names {
			column.Header = strings.TrimSpace(cell)
			column.Proposed, column.Confidence = suggestion(cell, claimed)
		}

		columns = append(columns, column)
	}

	return columns
}

func suggestion(header string, claimed map[string]bool) (string, string) {
	target := proposed(header)
	if target == "" {
		return targetIgnore, confidenceNone
	}

	confidence := confidenceLikely

	switch {
	case claimed[target]:
		confidence = confidenceAmbiguous
	case normalized(header) == target:
		confidence = confidenceCertain
	}

	claimed[target] = true

	return target, confidence
}

func delimiterNote(comma rune, guessed bool) string {
	if guessed {
		return fmt.Sprintf(
			"The columns look separated by %s. Say so in this run's settings if that is wrong: "+
				"read with the wrong separator, every row arrives as a single column.",
			delimiterNamed(comma),
		)
	}

	return fmt.Sprintf("This run reads the file with %s between its columns.", delimiterNamed(comma))
}

func delimiterNamed(comma rune) string {
	switch comma {
	case ';':
		return "a semicolon"
	case '\t':
		return "a tab"
	case '|':
		return "a pipe"
	default:
		return "a comma"
	}
}

func headerNote(names bool) string {
	if names {
		return noteHeaderRead
	}

	return noteHeaderMissing
}

func rowsNote(visible int, whole bool) string {
	if whole {
		return fmt.Sprintf("This file holds %s.", counted(visible))
	}

	return fmt.Sprintf(
		"The first %d KB of this file holds %s; the rest is counted as it is staged.",
		probeWindow>>10, counted(visible),
	)
}

func counted(rows int) string {
	if rows == 1 {
		return "1 row"
	}

	return strconv.Itoa(rows) + " rows"
}
