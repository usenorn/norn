package entity

import (
	"errors"
	"strings"
)

const rankDigits = "0123456789abcdefghijklmnopqrstuvwxyz"

const rankBase = len(rankDigits)

var (
	ErrIssueRankOutOfOrder = errors.New("issue rank bounds are out of order")
	ErrIssueRankMalformed  = errors.New("issue rank is malformed")
)

func ValidIssueRank(rank string) bool {
	if rank == "" {
		return false
	}

	if rank[len(rank)-1] == rankDigits[0] {
		return false
	}

	return strings.IndexFunc(rank, func(digit rune) bool {
		return !strings.ContainsRune(rankDigits, digit)
	}) < 0
}

func RankBetween(before, after string) (string, error) {
	if before != "" && !ValidIssueRank(before) {
		return "", ErrIssueRankMalformed
	}

	if after != "" && !ValidIssueRank(after) {
		return "", ErrIssueRankMalformed
	}

	if before != "" && after != "" && before >= after {
		return "", ErrIssueRankOutOfOrder
	}

	built := make([]byte, 0, len(before)+2)
	bounded := after != ""

	for position := 0; ; position++ {
		low := rankDigitAt(before, position)
		high := rankBase

		if bounded {
			high = rankDigitAt(after, position)
		}

		if low+1 < high {
			built = append(built, rankDigits[(low+high)/2])

			return string(built), nil
		}

		if low+1 == high {
			bounded = false
		}

		built = append(built, rankDigits[low])
	}
}

func rankDigitAt(rank string, position int) int {
	if position >= len(rank) {
		return 0
	}

	return strings.IndexByte(rankDigits, rank[position])
}
