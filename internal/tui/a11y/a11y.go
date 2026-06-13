package a11y

import "os"

func NoColor() bool {
	return os.Getenv("NO_COLOR") != ""
}

func Accessible() bool {
	return os.Getenv("ACCESSIBLE") != ""
}

func ReducedMotion() bool {
	return os.Getenv("NO_ANIMATE") != "" || Accessible() || NoColor()
}
