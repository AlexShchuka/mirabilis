package sandbox

import "errors"

func platformOpen(url string) ([]string, error) {
	for _, name := range []string{"xdg-open", "xdg_open"} {
		if p, err := lookPath(name); err == nil {
			return []string{p, url}, nil
		}
	}
	return nil, errors.New("sandbox: 'xdg-open' not found; set $BROWSER")
}
