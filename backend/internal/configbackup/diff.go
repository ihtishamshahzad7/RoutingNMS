package configbackup

import "strings"

// DiffLine is one line of a unified line-diff between two config versions.
type DiffLine struct {
	Op   string `json:"op"` // "equal" | "add" | "remove"
	Text string `json:"text"`
}

// LineDiff computes a minimal line-level diff between two config texts using
// the classic O(N*M) longest-common-subsequence table. Config exports are at
// most a few thousand lines, so the quadratic table is fine here and needs
// no external dependency (this repo has no diff library, matching the
// project's established preference for dependency-free tooling — see
// Feature 0.4's hand-rolled OpenAPI generator for the same pattern).
func LineDiff(oldText, newText string) []DiffLine {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	n, m := len(oldLines), len(newLines)

	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if oldLines[i] == newLines[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	out := make([]DiffLine, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case oldLines[i] == newLines[j]:
			out = append(out, DiffLine{Op: "equal", Text: oldLines[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, DiffLine{Op: "remove", Text: oldLines[i]})
			i++
		default:
			out = append(out, DiffLine{Op: "add", Text: newLines[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, DiffLine{Op: "remove", Text: oldLines[i]})
	}
	for ; j < m; j++ {
		out = append(out, DiffLine{Op: "add", Text: newLines[j]})
	}
	return out
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
