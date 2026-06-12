//go:build linux

package app_test

import "golang.org/x/sys/unix"

const reqGetTermios = unix.TCGETS
