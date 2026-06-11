package provision

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/config"
)

func home() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return h
}

func claudeDir() string { return filepath.Join(home(), ".claude") }

func settingsPath() string { return filepath.Join(claudeDir(), "settings.json") }

func readJSON(path string) (map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var m map[string]any
	dec := json.NewDecoder(f)
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func writeJSON(path string, m map[string]any) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

var seedManagedKeys = map[string]bool{
	"hooks":      true,
	"statusLine": true,
	"env":        true,
}

func mergeSettings(dest, seed map[string]any) map[string]any {
	out := make(map[string]any, len(dest))
	for k, v := range dest {
		out[k] = v
	}
	for k, sv := range seed {
		if seedManagedKeys[k] {
			out[k] = sv
			continue
		}
		if dv, ok := out[k]; ok {
			dm, dIsMap := dv.(map[string]any)
			sm, sIsMap := sv.(map[string]any)
			if dIsMap && sIsMap {
				out[k] = mergeSettings(dm, sm)
				continue
			}
		}
		if _, exists := out[k]; !exists {
			out[k] = sv
		}
	}
	return out
}

func EnsureSettings(cfg config.Config) error {
	cd := claudeDir()
	if err := os.MkdirAll(cd, 0o755); err != nil {
		return fmt.Errorf("provision: mkdir ~/.claude: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(cd, "xdg-data"), 0o755); err != nil {
		return fmt.Errorf("provision: mkdir ~/.claude/xdg-data: %w", err)
	}
	seed := cfg.SettingsSeed()
	if _, err := os.Stat(seed); err != nil {
		return nil
	}
	dest := settingsPath()
	if _, err := os.Stat(dest); err == nil {
		dm, derr := readJSON(dest)
		sm, serr := readJSON(seed)
		if derr == nil && serr == nil {
			merged := mergeSettings(dm, sm)
			delete(merged, "sandbox")
			if werr := writeJSON(dest, merged); werr != nil {
				warn("write settings; falling back to seed copy", werr)
				return copyFile(seed, dest)
			}
			return nil
		}
		fmt.Fprintf(os.Stderr, "[provision] WARN: parse settings for merge: dest=%v seed=%v; copying seed\n", derr, serr)
	}
	return copyFile(seed, dest)
}

func EnsureTheme(cfg config.Config) error {
	themeFile := themeFilePath()
	data, err := os.ReadFile(themeFile)
	if err != nil {
		return nil
	}
	th := string(data)
	if len(th) == 0 {
		return nil
	}

	for len(th) > 0 && (th[len(th)-1] == '\n' || th[len(th)-1] == '\r') {
		th = th[:len(th)-1]
	}
	if th == "" {
		return nil
	}
	dest := settingsPath()
	if _, err := os.Stat(dest); err != nil {
		return nil
	}
	m, err := readJSON(dest)
	if err != nil {
		warn("read settings for theme", err)
		return nil
	}
	m["theme"] = th
	if err := writeJSON(dest, m); err != nil {
		warn("write settings for theme", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
