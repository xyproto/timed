package timed

import (
	"fmt"
	"strings"
	"time"
)

type Transition struct {
	From         time.Time
	UpTo         time.Time
	FromFilename string
	ToFilename   string
	Type         string
}

func (t *Transition) Duration() time.Duration {
	return mod24(t.UpTo.Sub(t.From))
}

func (t *Transition) String(format string) string {
	if !strings.Contains(format, "%s") {
		// Return the verbose version, where type is always included and the filename is not reduced with a common string format
		if t.Type == "overlay" {
			return fmt.Sprintf("@%s-%s: %s .. %s", cFmt(t.From), cFmt(t.UpTo), t.FromFilename, t.ToFilename)
		}
		return fmt.Sprintf("@%s-%s: %s .. %s | %s", cFmt(t.From), cFmt(t.UpTo), t.FromFilename, t.ToFilename, t.Type)
	}
	fields := strings.SplitN(format, "%s", 2)
	prefix := fields[0]
	suffix := fields[1]
	if t.Type == "overlay" {
		return fmt.Sprintf("@%s-%s: %s .. %s", cFmt(t.From), cFmt(t.UpTo), t.FromFilename[len(prefix):len(t.FromFilename)-len(suffix)], t.ToFilename[len(prefix):len(t.ToFilename)-len(suffix)])
	}
	return fmt.Sprintf("@%s-%s: %s .. %s | %s", cFmt(t.From), cFmt(t.UpTo), t.FromFilename[len(prefix):len(t.FromFilename)-len(suffix)], t.ToFilename[len(prefix):len(t.ToFilename)-len(suffix)], t.Type)
}
