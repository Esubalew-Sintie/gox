package plan

import (
	"fmt"
	"strings"
)

const diffContext = 3

// Unified renders a unified diff between old and new content for display.
// It is meant for dry-run output, not for patch tools.
func Unified(path string, oldData, newData []byte) string {
	a := splitLines(oldData)
	b := splitLines(newData)
	edits := lcsEdits(a, b)

	var changed []int
	for i, e := range edits {
		if e.kind != ' ' {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- a/%s\n+++ b/%s\n", path, path)
	if oldData == nil {
		sb.Reset()
		fmt.Fprintf(&sb, "--- /dev/null\n+++ b/%s\n", path)
	}
	if newData == nil {
		sb.Reset()
		fmt.Fprintf(&sb, "--- a/%s\n+++ /dev/null\n", path)
	}

	start := max(changed[0]-diffContext, 0)
	end := min(changed[0]+diffContext+1, len(edits))
	for _, c := range changed[1:] {
		if c-diffContext <= end {
			end = min(c+diffContext+1, len(edits))
			continue
		}
		writeHunk(&sb, edits[start:end])
		start = max(c-diffContext, 0)
		end = min(c+diffContext+1, len(edits))
	}
	writeHunk(&sb, edits[start:end])
	return sb.String()
}

type edit struct {
	kind byte // ' ', '-', '+'
	text string
	ai   int // 0-based line in a (for ' ' and '-'; next a line for '+')
	bi   int // 0-based line in b (for ' ' and '+'; next b line for '-')
}

func writeHunk(sb *strings.Builder, hunk []edit) {
	var aCount, bCount int
	for _, e := range hunk {
		if e.kind != '+' {
			aCount++
		}
		if e.kind != '-' {
			bCount++
		}
	}
	aStart, bStart := hunk[0].ai+1, hunk[0].bi+1
	if aCount == 0 {
		aStart = hunk[0].ai
	}
	if bCount == 0 {
		bStart = hunk[0].bi
	}
	fmt.Fprintf(sb, "@@ -%d,%d +%d,%d @@\n", aStart, aCount, bStart, bCount)
	for _, e := range hunk {
		sb.WriteByte(e.kind)
		sb.WriteString(e.text)
		sb.WriteByte('\n')
	}
}

// lcsEdits returns the edit script from a to b. Beyond a size limit it
// degrades to a whole replacement, which is still correct for display.
func lcsEdits(a, b []string) []edit {
	n, m := len(a), len(b)
	var out []edit
	if n*m > 4_000_000 {
		for i, l := range a {
			out = append(out, edit{'-', l, i, 0})
		}
		for j, l := range b {
			out = append(out, edit{'+', l, n, j})
		}
		return out
	}
	dp := make([][]int32, n+1)
	for i := range dp {
		dp[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else {
				dp[i][j] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, edit{' ', a[i], i, j})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			out = append(out, edit{'-', a[i], i, j})
			i++
		default:
			out = append(out, edit{'+', b[j], i, j})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, edit{'-', a[i], i, j})
	}
	for ; j < m; j++ {
		out = append(out, edit{'+', b[j], i, j})
	}
	return out
}

func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	s := strings.TrimSuffix(string(data), "\n")
	return strings.Split(s, "\n")
}
