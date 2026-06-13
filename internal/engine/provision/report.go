package provision

import (
	"fmt"
	"os"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

func streamToStdout(events <-chan pipeline.Event) {
	for ev := range events {
		switch ev.Kind {
		case pipeline.EvSpawn:
			_, _ = fmt.Fprintf(os.Stdout, "+ %v\n", ev.Argv)
		case pipeline.EvLine:
			_, _ = fmt.Fprintln(os.Stdout, ev.Line)
		case pipeline.EvFailed:
			_, _ = fmt.Fprintf(os.Stdout, "[provision] FAIL %s: %v\n", ev.Step, ev.Err)
		case pipeline.EvSkipped:
			if ev.Err != nil {
				_, _ = fmt.Fprintf(os.Stdout, "[provision] WARN %s: %v\n", ev.Step, ev.Err)
			}
		}
	}
}
