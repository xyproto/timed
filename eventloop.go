package timed

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/anthonynsimon/bild/blend"
	"github.com/anthonynsimon/bild/imgio"
	"github.com/xyproto/event"
)

var setmut = &sync.RWMutex{}

// UntilNext finds the duration until the next event starts
func (fw *FatWallpaper) UntilNext(et time.Time) time.Duration {
	var startTimes []time.Time
	for _, t := range fw.Transitions {
		startTimes = append(startTimes, t.From)
	}
	for _, s := range fw.Statics {
		startTimes = append(startTimes, s.At)
	}
	mindiff := h24
	// OK, have all start times, now to find the ones that are both positive and smallest
	for _, st := range startTimes {
		//diff := st.Sub(et)
		diff := event.ToToday(et).Sub(event.ToToday(st))
		if diff > 0 && diff < mindiff {
			mindiff = diff
		}
	}
	return mindiff
}

// NextEvent finds the next event, given a timestamp.
// Returns an interface{} that is either a static or transition event.
func (fw *FatWallpaper) NextEvent(now time.Time) (interface{}, error) {
	// Create a map, from timestamps to wallpaper events
	events := make(map[time.Time]interface{})
	for _, t := range fw.Transitions {
		events[t.From] = t
	}
	for _, s := range fw.Statics {
		events[s.At] = s
	}
	if len(events) == 0 {
		return nil, errors.New("can not find next event: got no events")
	}
	// Go though all the event time stamps, and find the one that has the smallest (now time - event time)
	minDiff := h24
	var minEvent interface{}
	for t, e := range events {
		//fmt.Printf("now is: %v (%T)\n", now, now)
		//fmt.Printf("t is: %v (%T)\n", t, t)
		diff := event.ToToday(t).Sub(event.ToToday(now))
		//fmt.Println("Diff for", cFmt(t), ":", diff)
		if diff > 0 && diff < minDiff {
			minDiff = diff
			minEvent = e
			//fmt.Println("NEW SMALLEST DIFF RIGHT AFTER", cFmt(now), cFmt(t), minDiff)
		}
	}
	return minEvent, nil
}

// PrevEvent finds the previous event, given a timestamp.
// Returns an interface{} that is either a static or transition event.
func (fw *FatWallpaper) PrevEvent(now time.Time) (interface{}, error) {
	// Create a map, from timestamps to wallpaper events
	events := make(map[time.Time]interface{})
	for _, t := range fw.Transitions {
		events[t.From] = t
	}
	for _, s := range fw.Statics {
		events[s.At] = s
	}
	if len(events) == 0 {
		return nil, errors.New("can not find previous event: got no events")
	}
	// Go though all the event time stamps, and find the one that has the smallest (now time - event time)
	minDiff := h24
	var minEvent interface{}
	for t, e := range events {
		if minEvent == nil {
			minEvent = e
		}
		//fmt.Printf("now is: %v (%T)\n", now, now)
		//fmt.Printf("t is: %v (%T)\n", t, t)
		diff1 := event.ToToday(now).Sub(event.ToToday(t))
		diff2 := event.ToTomorrow(now).Sub(event.ToToday(t))
		//fmt.Println("Diff for", cFmt(t), ":", diff)
		if diff1 > 0 && diff1 < minDiff {
			minDiff = diff1
			minEvent = e
			//fmt.Println("NEW SMALLEST DIFF RIGHT BEFORE", cFmt(now), cFmt(t), minDiff)
		}
		if diff2 > 0 && diff2 < minDiff {
			minDiff = diff2
			minEvent = e
			//fmt.Println("NEW SMALLEST DIFF RIGHT BEFORE", cFmt(now), cFmt(t), minDiff)
		}

	}
	return minEvent, nil
}

// SetInitialWallpaper will set the first wallpaper, before starting the event loop
func (fw *FatWallpaper) SetInitialWallpaper(verbose bool, setWallpaperFunc func(string) error, tempImageFilename string) error {
	e, err := fw.PrevEvent(time.Now())
	if err != nil {
		return err
	}
	switch v := e.(type) {
	case *Static:
		s := v

		// Place values into variables, before enclosing it in the function below.
		from := s.At
		//elapsed := time.Now().Sub(s.At)
		elapsed := event.ToToday(time.Now()).Sub(event.ToToday(s.At))
		if elapsed < 0 {
			elapsed *= -1
		}
		window := mod24(fw.UntilNext(s.At) - elapsed) // duration until next event start, minus time elapsed
		cooldown := window

		imageFilename := s.Filename

		if verbose {
			fmt.Printf("Initial static wallpaper event at %s\n", cFmt(from))
			fmt.Println("Window:", dFmt(window))
			fmt.Println("Cooldown:", dFmt(cooldown))
			fmt.Println("Filename:", imageFilename)
		}

		// Find the absolute path
		absImageFilename, err := filepath.Abs(imageFilename)
		if err == nil {
			imageFilename = absImageFilename
		}

		// Check that the file exists
		if _, err := os.Stat(imageFilename); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", imageFilename)
		}

		// Set the desktop wallpaper, if possible
		if verbose {
			fmt.Printf("Setting %s.\n", imageFilename)
		}
		if err := setWallpaperFunc(imageFilename); err != nil {
			return fmt.Errorf("could not set wallpaper: %v", err)
		}

		// Just sleep for half the cooldown, to have some time to register events too
		if verbose {
			fmt.Println("Activating events in", dFmt(cooldown/2))
		}
		time.Sleep(cooldown / 2)
	case *Transition:
		t := v

		now := time.Now()
		window := t.Duration()
		progress := mod24(window - event.ToToday(t.UpTo).Sub(event.ToToday(now)))
		ratio := float64(progress) / float64(window)
		from := t.From
		steps := 10
		cooldown := window / time.Duration(steps)
		upTo := from.Add(window)
		tType := t.Type
		tFromFilename := t.FromFilename
		tToFilename := t.ToFilename
		loopWait := fw.LoopWait
		var err error

		if verbose {
			fmt.Printf("Initial transition event at %s (%d%% complete)\n", cFmt(from), int(ratio*100))
			fmt.Println("Progress:", dFmt(progress))
			fmt.Println("Up to:", cFmt(upTo))
			fmt.Println("Window:", dFmt(window))
			fmt.Println("Cooldown:", dFmt(cooldown))
			fmt.Println("Loop wait:", dFmt(loopWait))
			fmt.Println("Transition type:", tType)
			fmt.Println("From filename", tFromFilename)
			fmt.Println("To filename", tToFilename)
		}

		// Set the "from" image before crossfading, so that something happens immediately

		// Set the desktop wallpaper, if possible
		if verbose {
			fmt.Printf("Setting %s.\n", tFromFilename)
		}
		if err := setWallpaperFunc(tFromFilename); err != nil {
			return fmt.Errorf("could not set wallpaper: %v", err)
		}

		if verbose {
			fmt.Println("Crossfading between images.")
		}

		tFromImg, err := imgio.Open(tFromFilename)
		if err != nil {
			return err
		}

		tToImg, err := imgio.Open(tToFilename)
		if err != nil {
			return err
		}

		// Crossfade and write the new image to the temporary directory
		setmut.Lock()
		blendedImage := blend.Opacity(tFromImg, tToImg, ratio)
		err = imgio.Save(tempImageFilename, blendedImage, imgio.JPEGEncoder(100))
		if err != nil {
			setmut.Unlock()
			return fmt.Errorf("could not crossfade images in transition: %v", err)
		}
		setmut.Unlock()

		// Double check that the generated file exists
		if _, err := os.Stat(tempImageFilename); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", tempImageFilename)
		}

		// Set the desktop wallpaper, if possible
		if verbose {
			fmt.Printf("Setting %s.\n", tempImageFilename)
		}
		setmut.Lock()
		if err := setWallpaperFunc(tempImageFilename); err != nil {
			setmut.Unlock()
			return fmt.Errorf("could not set wallpaper: %v", err)
		}
		setmut.Unlock()

		// Just sleep for half the cooldown, to have some time to register events too
		if verbose {
			fmt.Println("Activating events in", dFmt(cooldown/2))
		}
		time.Sleep(cooldown / 2)
	default:
		return errors.New("could not set initial wallpaper: no previous event")
	}
	return nil
}

// EventLoop will start the event loop for this Simple Timed Wallpaper
func (fw *FatWallpaper) EventLoop(verbose bool, setWallpaperFunc func(string) error, tempImageFilename string) error {
	if verbose {
		if fw.Config != nil {
			fmt.Println("Using the GNOME Timed Wallpaper format")
		} else {
			fmt.Println("Using the Simple Timed Wallpaper format.")
		}
	}

	var err error
	initialW := fw
	if fw.Config != nil {
		initialW, err = GnomeToSimple(fw)
		if err != nil {
			return err
		}
	}

	// Listen for SIGHUP or SIGUSR1, to refresh the wallpaper.
	// Can be used after resume from sleep.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGUSR1)
	go func() {
		for {
			// Wait for a signal of the type given to signal.Notify
			sig := <-signals
			// Refresh the wallpaper
			fmt.Println("Received signal", sig)
			// Launch a goroutine for setting the wallpaper
			go func() {
				setmut.Lock()
				// Convert to a SimpleTimedWallpaper, only for setting the initial wallpaper

				if err := initialW.SetInitialWallpaper(verbose, setWallpaperFunc, tempImageFilename); err != nil {
					fmt.Fprintln(os.Stderr, "Error:", err)
				}
				setmut.Unlock()
			}()
		}
	}()

	setmut.Lock()
	if err := initialW.SetInitialWallpaper(verbose, setWallpaperFunc, tempImageFilename); err != nil {
		setmut.Unlock()
		return err
	}
	setmut.Unlock()

	eventloop := event.NewLoop()

	if fw.Config != nil {

		// Get the start time for the wallpaper collection (which is offset by X
		// seconds per static wallpaper)
		startTime := fw.StartTime()

		// The start time of the timed wallpaper as a whole
		if verbose {
			fmt.Println("Timed wallpaper start time:", cFmt(startTime))
		}

		totalElements := len(fw.Config.Statics) + len(fw.Config.Transitions)

		// Keep track of the total time. It is increased every time a new element duration is encountered.
		eventTime := startTime

		for i := 0; i < totalElements; i++ {
			// The duration of the event is specified in the XML file, but not when it should start

			// Get an element, by index. This is an interface{} and is expected to be a GStatic or a GTransition
			eInterface, err := fw.Config.Get(i)
			if err != nil {
				return err
			}
			if s, ok := eInterface.(GStatic); ok {
				if verbose {
					fmt.Printf("Registering static event at %s for setting %s\n", cFmt(eventTime), s.Filename)
				}

				// Place values into variables, before enclosing it in the function below.
				from := eventTime
				window := s.Duration()
				cooldown := window
				imageFilename := s.Filename

				// Register a static event
				eventloop.Add(event.New(from, window, cooldown, func() {
					if verbose {
						fmt.Printf("Triggered static wallpaper event at %s\n", cFmt(from))
						fmt.Println("Window:", dFmt(window))
						fmt.Println("Cooldown:", dFmt(cooldown))
						fmt.Println("Filename:", imageFilename)
					}

					// Find the absolute path
					absImageFilename, err := filepath.Abs(imageFilename)
					if err == nil {
						imageFilename = absImageFilename
					}

					// Check that the file exists
					if _, err := os.Stat(imageFilename); os.IsNotExist(err) {
						fmt.Fprintf(os.Stderr, "File does not exist: %s\n", imageFilename)
						return // return from anon func
					}

					// Set the desktop wallpaper, if possible
					if verbose {
						fmt.Printf("Setting %s.\n", imageFilename)
					}
					if err := setWallpaperFunc(imageFilename); err != nil {
						fmt.Fprintf(os.Stderr, "Could not set wallpaper: %v\n", err)
						return // return from anon func
					}
				}))

				// Increase the variable that keeps track of the time
				eventTime = eventTime.Add(window)

			} else if t, ok := eInterface.(GTransition); ok {
				if verbose {
					fmt.Printf("Registering transition at %s for transitioning from %s to %s.\n", cFmt(eventTime), t.FromFilename, t.ToFilename)
				}

				// cross fade steps
				steps := 10

				from := eventTime
				window := t.Duration()
				upTo := eventTime.Add(window)
				cooldown := window / time.Duration(steps)
				tType := t.Type
				tFromFilename := t.FromFilename
				tToFilename := t.ToFilename
				loopWait := fw.LoopWait

				// Register a transition event
				eventloop.Add(event.New(from, window, cooldown, func() {
					progress := mod24(window - event.ToToday(upTo).Sub(event.ToToday(time.Now())))
					ratio := float64(progress) / float64(window)

					if verbose {
						fmt.Printf("Triggered transition event at %s (%d%% complete)\n", cFmt(from), int(ratio*100))
						fmt.Println("Progress:", dFmt(progress))
						fmt.Println("Up to:", cFmt(upTo))
						fmt.Println("Window:", dFmt(window))
						fmt.Println("Cooldown:", dFmt(cooldown))
						fmt.Println("Loop wait:", dFmt(loopWait))
						fmt.Println("Transition type:", tType)
						fmt.Println("From filename", tFromFilename)
						fmt.Println("To filename", tToFilename)
					}

					if verbose {
						fmt.Println("Crossfading between images.")
					}

					// Crossfade and write the new image to the temporary directory
					tFromImg, err := imgio.Open(tFromFilename)
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						return
					}

					tToImg, err := imgio.Open(tToFilename)
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						return
					}

					// Crossfade and write the new image to the temporary directory
					setmut.Lock()
					blendedImage := blend.Opacity(tFromImg, tToImg, ratio)
					err = imgio.Save(tempImageFilename, blendedImage, imgio.JPEGEncoder(100))
					if err != nil {
						fmt.Fprintf(os.Stderr, "Could not crossfade images in transition: %v\n", err)
						setmut.Unlock()
						return
					}
					setmut.Unlock()

					// Double check that the generated file exists
					if _, err := os.Stat(tempImageFilename); os.IsNotExist(err) {
						fmt.Fprintf(os.Stderr, "File does not exist: %s\n", tempImageFilename)
						return // return from anon func
					}

					// Set the desktop wallpaper, if possible
					if verbose {
						fmt.Printf("Setting %s.\n", tempImageFilename)
					}
					if err := setWallpaperFunc(tempImageFilename); err != nil {
						fmt.Fprintf(os.Stderr, "Could not set wallpaper: %v\n", err)
						return // return from anon func
					}

				}))

				// Increase the variable that keeps track of the time
				eventTime = eventTime.Add(window)
			} else {
				// This should never happen, it would be an implementation error
				panic("got an element that is not a GStatic and not a GTransition")
			}
		}

		// Endless loop! Will wait loopWait duration between each event loop cycle.
		eventloop.Go(fw.LoopWait)

	} else {

		for _, s := range fw.Statics {
			if verbose {
				fmt.Printf("Registering static event at %s for setting %s\n", cFmt(s.At), s.Filename)
			}

			// Place values into variables, before enclosing it in the function below.
			from := s.At
			window := mod24(fw.UntilNext(s.At)) // duration until next event start
			cooldown := window
			imageFilename := s.Filename

			// Register a static event
			eventloop.Add(event.New(from, window, cooldown, func() {
				if verbose {
					fmt.Printf("Triggered static wallpaper event at %s\n", cFmt(from))
					fmt.Println("Window:", dFmt(window))
					fmt.Println("Cooldown:", dFmt(cooldown))
					fmt.Println("Filename:", imageFilename)
				}

				// Find the absolute path
				absImageFilename, err := filepath.Abs(imageFilename)
				if err == nil {
					imageFilename = absImageFilename
				}

				// Check that the file exists
				if _, err := os.Stat(imageFilename); os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "File does not exist: %s\n", imageFilename)
					return // return from anon func
				}

				// Set the desktop wallpaper, if possible
				if verbose {
					fmt.Printf("Setting %s.\n", imageFilename)
				}
				if err := setWallpaperFunc(imageFilename); err != nil {
					fmt.Fprintf(os.Stderr, "Could not set wallpaper: %v\n", err)
					return // return from anon func
				}
			}))
		}
		for _, t := range fw.Transitions {
			if verbose {
				fmt.Printf("Registering transition at %s for transitioning from %s to %s.\n", cFmt(t.From), t.FromFilename, t.ToFilename)
			}

			// cross fade steps
			steps := 10

			// Set variables
			from := t.From
			window := t.Duration()
			cooldown := window / time.Duration(steps)
			upTo := from.Add(window)
			tType := t.Type
			tFromFilename := t.FromFilename
			tToFilename := t.ToFilename
			loopWait := fw.LoopWait

			// Register a transition event
			//eventloop.Add(event.New(from, window, cooldown, event.ProgressWrapperInterval(from, upTo, loopWait, func(ratio float64) {
			eventloop.Add(event.New(from, window, cooldown, func() {
				progress := mod24(window - event.ToToday(upTo).Sub(event.ToToday(time.Now())))
				ratio := float64(progress) / float64(window)

				if verbose {
					fmt.Printf("Triggered transition event at %s (%d%% complete)\n", cFmt(from), int(ratio*100))
					fmt.Println("Progress:", dFmt(progress))
					fmt.Println("Up to:", cFmt(upTo))
					fmt.Println("Window:", dFmt(window))
					fmt.Println("Cooldown:", dFmt(cooldown))
					fmt.Println("Loop wait:", dFmt(loopWait))
					fmt.Println("Transition type:", tType)
					fmt.Println("From filename", tFromFilename)
					fmt.Println("To filename", tToFilename)
				}

				if verbose {
					fmt.Println("Crossfading between images.")
				}

				// Crossfade and write the new image to the temporary directory
				tFromImg, err := imgio.Open(tFromFilename)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return
				}

				tToImg, err := imgio.Open(tToFilename)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					return
				}

				// Crossfade and write the new image to the temporary directory
				setmut.Lock()
				blendedImage := blend.Opacity(tFromImg, tToImg, ratio)
				err = imgio.Save(tempImageFilename, blendedImage, imgio.JPEGEncoder(100))
				if err != nil {
					fmt.Fprintf(os.Stderr, "Could not crossfade images in transition: %v\n", err)
					setmut.Unlock()
					return
				}
				setmut.Unlock()

				// Double check that the generated file exists
				if _, err := os.Stat(tempImageFilename); os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "File does not exist: %s\n", tempImageFilename)
					return // return from anon func
				}

				// Set the desktop wallpaper, if possible
				if verbose {
					fmt.Printf("Setting %s.\n", tempImageFilename)
				}
				setmut.Lock()
				if err := setWallpaperFunc(tempImageFilename); err != nil {
					setmut.Unlock()
					fmt.Fprintf(os.Stderr, "Could not set wallpaper: %v\n", err)
					return // return from anon func
				}
				setmut.Unlock()
			}))
		}

		// Endless loop! Will wait LoopWait duration between each event loop cycle.
		eventloop.Go(fw.LoopWait)
	}

	return nil
}
