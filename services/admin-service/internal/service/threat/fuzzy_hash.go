package threat

import (
	"github.com/glaslos/tlsh"
)

// CalculateFuzzyHash generates a TLSH hash for the given HTML content.
// It requires at least 50 bytes of input; otherwise, it returns an empty string.
func CalculateFuzzyHash(html []byte) string {
	if len(html) < 50 {
		return ""
	}
	hash, err := tlsh.HashBytes(html)
	if err != nil || hash == nil {
		return ""
	}
	return hash.String()
}

// CompareFuzzyHash compares two TLSH hashes and returns a similarity score from 0.0 to 100.0.
// A distance of 0 means identical (100% similarity).
// TLSH distances typically range up to ~300. We invert and scale it so >=100 distance is 0%.
func CompareFuzzyHash(hash1, hash2 string) float64 {
	if hash1 == "" || hash2 == "" {
		return 0
	}

	h1, err := tlsh.ParseStringToTlsh(hash1)
	if err != nil {
		return 0
	}
	h2, err := tlsh.ParseStringToTlsh(hash2)
	if err != nil {
		return 0
	}

	diff := h1.Diff(h2)

	// Invert the difference to a similarity percentage.
	// TLSH distance: 0 = identical, higher = more different.
	// A threshold of 30 or less is strongly similar.
	// A threshold of 100 is essentially different.
	if diff == 0 {
		return 100.0
	}
	if diff >= 100 {
		return 0.0
	}

	similarity := 100.0 - float64(diff)
	if similarity < 0 {
		similarity = 0
	}
	return similarity
}
