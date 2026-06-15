// Package reconcile installs the subset of wanted items not already present.
package reconcile

import "errors"

func Missing[T comparable](want []T, have map[T]bool, install func(T) error) error {
	var errs []error
	for _, w := range want {
		if have[w] {
			continue
		}
		if err := install(w); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
