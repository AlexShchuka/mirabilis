package hooks

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ecosystemDirRel       = "ecosystem"
	ecosystemIndexByteCap = 16384
	ecosystemHeadLines    = 3
)

func ecosystemRoot() string { return filepath.Join(home(), ecosystemDirRel) }

func joinContext(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, "\n")
}

func ecosystemContext() string {
	var parts []string
	if idx := darwinMemoryIndex(); idx != "" {
		parts = append(parts, idx)
	}
	if shield := shieldMemoryIndex(); shield != "" {
		parts = append(parts, shield)
	}
	return strings.Join(parts, "\n\n")
}

func darwinMemoryIndex() string {
	path := filepath.Join(ecosystemRoot(), "darwin", "ecosystem", "MEMORY-index.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	body := strings.TrimSpace(string(data))
	if body == "" {
		return ""
	}
	if len(body) > ecosystemIndexByteCap {
		body = body[:ecosystemIndexByteCap]
	}
	return "# Ecosystem memory (darwin)\n\n" + body
}

func shieldMemoryIndex() string {
	dir := filepath.Join(ecosystemRoot(), "SolitaryEquilibriumShield", "memory")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		names = append(names, e.Name())
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)

	var sb strings.Builder
	sb.WriteString("# Ecosystem memory (SolitaryEquilibriumShield)\n\n")
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		sb.WriteString("- memory/" + name)
		if head := headLines(data, ecosystemHeadLines); head != "" {
			sb.WriteString(" — " + head)
		}
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func headLines(data []byte, n int) string {
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line == "---" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
		if len(lines) >= n {
			break
		}
	}
	return strings.Join(lines, " ")
}
