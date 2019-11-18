package timed

import (
	"fmt"
	"strings"
	"time"
)

type Static struct {
	At       time.Time
	Filename string
}

func (s *Static) String(format string) string {
	if !strings.Contains(format, "%s") {
		// Return the verbose version, where type is always included and the filename is not reduced with a common string format
		return fmt.Sprintf("@%s: %s", cFmt(s.At), s.Filename)
	}
	fields := strings.SplitN(format, "%s", 2)
	prefix := fields[0]
	suffix := fields[1]
	return fmt.Sprintf("@%s: %s", cFmt(s.At), s.Filename[len(prefix):len(s.Filename)-len(suffix)])
}
