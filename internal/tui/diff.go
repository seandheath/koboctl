package tui

import "strings"

// diffLine is one line of a unified-style diff.
type diffLine struct {
	kind byte // ' ' context, '+' added, '-' removed
	text string
}

// lineDiff computes a minimal line-level diff between old and new text via a
// longest-common-subsequence backtrace. Small inputs (a rendered manifest is a
// few dozen lines), so the O(n·m) DP is fine and avoids a diff dependency.
func lineDiff(oldText, newText string) []diffLine {
	a := strings.Split(oldText, "\n")
	b := strings.Split(newText, "\n")
	n, m := len(a), len(b)

	// lcs[i][j] = LCS length of a[i:] and b[j:].
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var out []diffLine
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, diffLine{' ', a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			out = append(out, diffLine{'-', a[i]})
			i++
		default:
			out = append(out, diffLine{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, diffLine{'-', a[i]})
	}
	for ; j < m; j++ {
		out = append(out, diffLine{'+', b[j]})
	}
	return out
}

// changedOnly filters a diff to only the added/removed lines (drops context).
func changedOnly(d []diffLine) []diffLine {
	var out []diffLine
	for _, l := range d {
		if l.kind != ' ' {
			out = append(out, l)
		}
	}
	return out
}
