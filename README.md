# Timed

This is a package for dealing with the GNOME Timed Wallpaper XML and Simple Timed Wallpaper (STW) files.

# The Simple Timed Wallpaper Format

STW is a format for a configuration file that specifies in which time ranges wallpapers should change from one to another, and with which transition.

It's a similar to the GNOME timed wallpaper XML format, but much simpler and less verbose.

## Specification

### Version 1.0.0

* [Markdown](https://github.com/xyproto/timed/blob/master/stw-1.0.0.md)
* [PDF](https://github.com/xyproto/timed/raw/master/stw-1.0.0.pdf)

## Go module

[![GoDoc](https://godoc.org/github.com/xyproto/timed?status.svg)](https://godoc.org/github.com/xyproto/timed)

The `timed` Go module can be used for parsing the file format and for running an event loop for setting the wallpaper, given a function with this signature:

```go
func(string) error
```

Where the given string is the image filename to be set.

# General info

* Version: 0.1.0
* License: MIT
* Author: Alexander F. Rødseth &lt;xyproto@archlinux.org&gt;
