package sandbox

import "errors"

func platformOpen(url string) ([]string, error) {
	code, err := lookPath("open")
	if err != nil {
		return nil, errors.New("sandbox: 'open' not found; set $BROWSER")
	}
	return []string{code, url}, nil
}
