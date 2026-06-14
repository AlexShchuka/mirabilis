package hooks

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexShchuka/mirabilis/internal/engine/config"
)

type memoryMeta struct {
	category   string
	memoryType string
	summary    string
}

func parseFrontmatter(data []byte) memoryMeta {
	var m memoryMeta
	sc := bufio.NewScanner(bytes.NewReader(data))
	inFront := false
	for sc.Scan() {
		line := sc.Text()
		if line == "---" {
			if !inFront {
				inFront = true
				continue
			}
			break
		}
		if !inFront {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "category: "); ok {
			m.category = strings.TrimSpace(rest)
		}
		if rest, ok := strings.CutPrefix(line, "memory_type: "); ok {
			m.memoryType = strings.TrimSpace(rest)
		}
		if rest, ok := strings.CutPrefix(line, "summary: "); ok {
			m.summary = strings.TrimSpace(rest)
		}
	}
	return m
}

func countInvariants(data []byte) int {
	n := 0
	sc := bufio.NewScanner(bytes.NewReader(data))
	inFront := false
	pastFront := false
	for sc.Scan() {
		line := sc.Text()
		if line == "---" {
			if !inFront {
				inFront = true
				continue
			}
			pastFront = true
			inFront = false
			continue
		}
		if pastFront && strings.HasPrefix(line, "- ") {
			n++
		}
	}
	return n
}

func readBullets(data []byte) []string {
	var bullets []string
	sc := bufio.NewScanner(bytes.NewReader(data))
	pastFront := false
	inFront := false
	for sc.Scan() {
		line := sc.Text()
		if line == "---" {
			if !inFront {
				inFront = true
				continue
			}
			pastFront = true
			inFront = false
			continue
		}
		if pastFront && strings.HasPrefix(line, "- ") {
			bullets = append(bullets, line)
		}
	}
	return bullets
}

func memoryIndex(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil
	}

	type fileInfo struct {
		meta     memoryMeta
		fileName string
		count    int
		data     []byte
	}

	knownCats := make(map[string]bool, len(config.MemoryCategories))
	for _, cat := range config.MemoryCategories {
		knownCats[cat.Name] = true
	}

	byCategory := make(map[string]fileInfo, len(config.MemoryCategories))
	var extras []fileInfo

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" || e.Name() == "MEMORY.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		meta := parseFrontmatter(data)
		count := countInvariants(data)
		fi := fileInfo{meta: meta, count: count, fileName: e.Name(), data: data}
		if knownCats[meta.category] {
			if _, exists := byCategory[meta.category]; !exists {
				byCategory[meta.category] = fi
			}
		} else {
			if meta.category == "" {
				stem := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
				fi.meta.category = stem
			}
			extras = append(extras, fi)
		}
	}

	var sb strings.Builder
	sb.WriteString("# Sandbox memory index\n\n")
	for _, cat := range config.MemoryCategories {
		fi, ok := byCategory[cat.Name]
		if !ok {
			continue
		}
		if fi.meta.category == "sandbox-ops" {
			fmt.Fprintf(&sb, "## sandbox-ops\n\n")
			for _, bullet := range readBullets(fi.data) {
				sb.WriteString(bullet + "\n")
			}
			sb.WriteString("\n")
			continue
		}
		fmt.Fprintf(&sb, "- **%s** (%s, %d) — %s  · memory/%s\n",
			fi.meta.category, fi.meta.memoryType, fi.count, fi.meta.summary, fi.fileName)
	}
	for _, fi := range extras {
		fmt.Fprintf(&sb, "- **%s** (%s, %d) — %s  · memory/%s\n",
			fi.meta.category, fi.meta.memoryType, fi.count, fi.meta.summary, fi.fileName)
	}
	return sb.String(), nil
}

func tokenize(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', ':', '/', '\\', '.', ',', ';', '!', '?', '"', '\'', '(', ')', '[', ']', '{', '}', '=', '-', '_':
			return true
		}
		return false
	})
	var out []string
	for _, f := range fields {
		if len(f) >= 3 {
			out = append(out, strings.ToLower(f))
		}
	}
	return out
}

func matchesBullet(bullet string, tokens []string) bool {
	low := strings.ToLower(bullet)
	for _, tok := range tokens {
		if strings.Contains(low, tok) {
			return true
		}
	}
	return false
}
