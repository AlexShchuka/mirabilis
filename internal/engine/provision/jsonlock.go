package provision

import (
	"os"
	"path/filepath"
	"sync"
)

var (
	pathMutexesMu sync.Mutex
	pathMutexes   = map[string]*sync.Mutex{}
)

func pathMutex(path string) *sync.Mutex {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	pathMutexesMu.Lock()
	defer pathMutexesMu.Unlock()
	mu, ok := pathMutexes[abs]
	if !ok {
		mu = &sync.Mutex{}
		pathMutexes[abs] = mu
	}
	return mu
}

func updateJSON(path string, mutate func(map[string]any) error) error {
	mu := pathMutex(path)
	mu.Lock()
	defer mu.Unlock()

	m := map[string]any{}
	if existing, rerr := readJSON(path); rerr == nil {
		m = existing
	} else if !os.IsNotExist(rerr) {
		return rerr
	}
	if err := mutate(m); err != nil {
		return err
	}
	return writeJSON(path, m)
}
