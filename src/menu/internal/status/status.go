package status

import (
	"encoding/json"
	"io"
	"os"
)

type Status struct {
	CommitsBehind int    `json:"commitsBehind"`
	Stale         bool   `json:"stale"`
	Harness       string `json:"harness"`
}

func FromStdin() Status {
	var s Status
	fi, err := os.Stdin.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) != 0 {
		return s
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		return s
	}
	_ = json.Unmarshal(data, &s)
	return s
}
