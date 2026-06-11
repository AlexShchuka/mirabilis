package provision

import (
	"os"
	"path/filepath"

	"github.com/AlexShchuka/mirabilis/internal/runtime"
	"github.com/AlexShchuka/mirabilis/internal/tgtoken"
)

const FileTelegramToken = tgtoken.Filename

func TelegramTokenPath() string {
	return filepath.Join(claudeDir(), FileTelegramToken)
}

func WriteTelegramToken(token string) error {
	_ = runtime.KeychainStore("telegram-token", token)
	return writeTokenFile(token)
}

func writeTokenFile(token string) error {
	cd := claudeDir()
	if err := os.MkdirAll(cd, 0o755); err != nil {
		return err
	}
	dest := TelegramTokenPath()
	tmp, err := os.CreateTemp(cd, ".tgtoken-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(token + "\n"); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dest)
}

func ReadTelegramTokenFile() string {
	return tgtoken.Read()
}
