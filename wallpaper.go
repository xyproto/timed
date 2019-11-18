package timed

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
	"time"
)

var defaultEventLoopDelay = 5 * time.Second

// FatWallpaper contains all data for either a Simple Timed Wallpaper or a GNOME Timed Wallpaper
type FatWallpaper struct {
	GNOME       bool
	Version     string
	Name        string
	Format      string
	Path        string // not part of the file data, but handy when parsing
	Statics     []*Static
	Transitions []*Transition
	LoopWait    time.Duration // how long the main event loop should sleep
	Config      *GBackground  // set to nil when not a GNOME timed wallpaper
}

// NewGnome creates a new Gnome Timed Wallpaper struct
func NewGnome(name, path string, config *GBackground) *FatWallpaper {
	return &FatWallpaper{GNOME: true, Name: name, Path: path, Config: config, LoopWait: defaultEventLoopDelay}
}

// NewSimple creates a new Simple Timed Wallpaper struct
func NewSimple(version, name, format string) *FatWallpaper {
	var (
		statics     []*Static
		transitions []*Transition
	)
	return &FatWallpaper{GNOME: false, Version: version, Name: name, Format: format, Path: "", Statics: statics, Transitions: transitions, LoopWait: defaultEventLoopDelay}
}

// StartTime returns the timed wallpaper start time, as a time.Time
func (fw *FatWallpaper) StartTime() time.Time {
	if !fw.GNOME {
		panic("not implemented for STW")
	}
	// gtw.Config.StartTime is a struct that contains ints,
	// where the values are directly from the parsed XML.
	st := fw.Config.StartTime
	return time.Date(st.Year, time.Month(st.Month), st.Day, st.Hour, st.Minute, 0, 0, time.Local)
}

func (fw *FatWallpaper) Images() []string {
	if !fw.GNOME {
		// STW
		panic("not implemented for STW")
	}
	var filenames []string
	for _, static := range fw.Config.Statics {
		filenames = append(filenames, static.Filename)
	}
	for _, transition := range fw.Config.Transitions {
		filenames = append(filenames, transition.FromFilename)
		filenames = append(filenames, transition.ToFilename)
	}
	return unique(filenames)
}

// String builds a string with various information about this GNOME timed wallpaper
func (fw *FatWallpaper) String() string {
	if fw.GNOME {
		var sb strings.Builder
		sb.WriteString("path\t\t\t= ")
		sb.WriteString(fw.Path)
		sb.WriteString("\nstart time\t\t= ")
		sb.WriteString(fw.StartTime().String())
		sb.WriteString("\nnumber of static tags\t= ")
		sb.WriteString(strconv.Itoa(len(fw.Config.Statics)))
		sb.WriteString("\nnumber of transitions\t= ")
		sb.WriteString(strconv.Itoa(len(fw.Config.Transitions)))
		sb.WriteString("\nuses these images:\n")
		for _, filename := range fw.Images() {
			sb.WriteString("\t" + filename + "\n")
		}
		return strings.TrimSpace(sb.String())
	} else {
		var lines []string
		for _, s := range fw.Statics {
			lines = append(lines, s.String(fw.Format))
		}
		for _, t := range fw.Transitions {
			lines = append(lines, t.String(fw.Format))
		}
		sort.Strings(lines)
		return fmt.Sprintf("stw: %s\nname: %s\nformat: %s\n", fw.Version, fw.Name, fw.Format) + strings.Join(lines, "\n")
	}
}

func (fw *FatWallpaper) AddStatic(at time.Time, filename string) {
	if fw.GNOME {
		panic("not implemented for GNOME timed wallpaper")
	}
	var s Static
	s.At = at
	if len(fw.Format) > 0 {
		s.Filename = fmt.Sprintf(fw.Format, filename)
	} else {
		s.Filename = filename
	}
	fw.Statics = append(fw.Statics, &s)
}

func (fw *FatWallpaper) AddTransition(from, upto time.Time, fromFilename, toFilename, transitionType string) {
	if fw.GNOME {
		panic("not implemented for GNOME timed wallpaper")
	}
	var t Transition
	t.From = from
	t.UpTo = upto
	if len(fw.Format) > 0 {
		t.FromFilename = fmt.Sprintf(fw.Format, fromFilename)
		t.ToFilename = fmt.Sprintf(fw.Format, toFilename)
	} else {
		t.FromFilename = fromFilename
		t.ToFilename = toFilename
	}
	if len(transitionType) == 0 {
		t.Type = "overlay"
	} else {
		t.Type = transitionType
	}
	fw.Transitions = append(fw.Transitions, &t)
}

func ParseSTW(filename string) (*FatWallpaper, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return DataToSimple(filename, data)
}

// DataToSimple converts from the contents of a Simple Timed Wallpaper file to
// a Wallpaper structs. The given path is used in the error messages
// and for setting stw.Path.
func DataToSimple(path string, data []byte) (*FatWallpaper, error) {
	var ts []*Transition
	var ss []*Static
	parsed := make(map[string]string)
	for lineCount, byteLine := range bytes.Split(data, []byte("\n")) {
		trimmed := strings.TrimSpace(string(byteLine))
		if strings.HasPrefix(trimmed, "#") {
			//fmt.Fprintf(os.Stderr, trimmed[1:])
			continue
		} else if strings.HasPrefix(trimmed, "//") {
			//fmt.Fprintf(os.Stderr, trimmed[2:])
			continue
		} else if len(trimmed) == 0 {
			continue
		}
		if strings.HasPrefix(trimmed, "@") {
			if len(trimmed) > 6 && (trimmed[6] == ' ' || trimmed[6] == '-') && (trimmed[7] != ':') {
				if strings.Count(trimmed, "-") < 1 {
					return nil, fmt.Errorf("could not parse %s (no dash), line %d: %s", path, lineCount, trimmed)
				}
				fields := strings.SplitN(trimmed[1:], "-", 2)
				time1 := strings.TrimSpace(fields[0])
				if strings.Count(fields[1], ":") < 2 {
					return nil, fmt.Errorf("could not parse %s (missing colon), line %d: %s", path, lineCount, trimmed)
				}
				fields = strings.SplitN(fields[1], ":", 3)
				time2 := strings.TrimSpace(fields[0] + ":" + fields[1])
				filenames := fields[2]
				if !strings.Contains(filenames, "..") {
					return nil, fmt.Errorf("could not parse %s (missing \"..\"), line %d: %s", path, lineCount, trimmed)
				}
				fields = strings.SplitN(filenames, "..", 2)
				filename1 := strings.TrimSpace(fields[0])
				filename2 := strings.TrimSpace(fields[1])
				transitionType := "overlay"
				if strings.Contains(filename2, "|") {
					fields := strings.SplitN(filename2, "|", 2)
					filename2 = strings.TrimSpace(fields[0])
					transitionType = strings.TrimSpace(fields[1])
				}
				//fmt.Println("TRANSITION", time1, "|", time2, "|", filename1, "|", filename2, "|", transitionType)
				t1, err := time.Parse("15:04", time1)
				if err != nil {
					return nil, fmt.Errorf("could not parse %s (time), line %d: %s", path, lineCount, trimmed)
				}
				t2, err := time.Parse("15:04", time2)
				if err != nil {
					return nil, fmt.Errorf("could not parse %s (time), line %d: %s", path, lineCount, trimmed)
				}
				ts = append(ts, &Transition{t1, t2, filename1, filename2, transitionType})
			} else {
				if strings.Count(trimmed, ":") < 2 {
					return nil, fmt.Errorf("could not parse %s (missing colon), line %d: %s", path, lineCount, trimmed)
				}
				fields := strings.SplitN(trimmed[1:], ":", 3)
				time1 := strings.TrimSpace(fields[0] + ":" + fields[1])
				filename := strings.TrimSpace(fields[2])
				//fmt.Println("STATIC", time1, "|", filename)
				t1, err := time.Parse("15:04", time1)
				if err != nil {
					return nil, fmt.Errorf("could not parse %s (time), line %d: %s", path, lineCount, trimmed)
				}
				ss = append(ss, &Static{t1, filename})
			}
		} else if strings.Contains(trimmed, ":") {
			//fmt.Println("FIELD", trimmed)
			if strings.Count(trimmed, ":") < 1 {
				return nil, fmt.Errorf("could not parse %s (missing colon), line %d: %s", path, lineCount, trimmed)
			}
			fields := strings.SplitN(trimmed, ":", 2)
			key := strings.TrimSpace(fields[0])
			value := strings.TrimSpace(fields[1])
			parsed[key] = value
		} else {
			return nil, fmt.Errorf("could not parse %s (invalid syntax), line %d: %s", path, lineCount, trimmed)
		}
	}
	version, ok := parsed["stw"]
	if !ok {
		return nil, fmt.Errorf("could not find stw field in %s", path)
	}
	name := parsed["name"]     // optional
	format := parsed["format"] // optional

	stw := NewSimple(version, name, format)
	stw.Path = path
	for _, t := range ts {
		// Adding transitions in a way that make sure the format string is used when interpreting the filenames
		stw.AddTransition(t.From, t.UpTo, t.FromFilename, t.ToFilename, t.Type)
	}
	for _, s := range ss {
		// Adding static images in a way that make sure the format string is used when interpreting the filenames
		stw.AddStatic(s.At, s.Filename)
	}
	//fmt.Println(stw)
	return stw, nil
}
