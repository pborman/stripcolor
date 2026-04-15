//   Copyright 2026 Paul Borman
//   
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//      
//       http://www.apache.org/licenses/LICENSE-2.0
//      
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

// Program stripcolor runs the provided program on a pseudo TTY and strips all
// color changes.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty/v2"
	"github.com/pborman/ansi"
	"golang.org/x/crypto/ssh/terminal"
)

const debug = false // set to true when debugging escape sequences
var log *os.File    // where to log output when debug is true

func main() {
	if debug {
		var err error
		log, err = os.Create("log")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, "Usage: stripcolor COMMAND ...\n")
		os.Exit(1)
	}
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGWINCH)
	ws, err := pty.GetsizeFull(os.Stdout)
	if err == nil {
		ostate, err := terminal.MakeRaw(0)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer func() {
			terminal.Restore(0, ostate)
		}()
	}

	tty, err := pty.StartWithSize(exec.Command(os.Args[1], os.Args[2:]...), ws)
	go func() {
		for range ch {
			pty.InheritSize(tty, os.Stdout)
		}
	}()
	go io.Copy(tty, os.Stdin)
	start := 0
	for {
		var buf [1024]byte
		r, err := tty.Read(buf[start:])
		if r > 0 {
			filter(buf[:r], os.Stdout)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
	}
}

// strip returns true if s should be stripped from the output.
func strip(s *ansi.S) bool {
	if s.Code == ansi.SGR {
		// Do not strip out Reset reset
		if s.Params[0] == "0" {
			return false
		}

		// This is aggressive pruning.  This could be modified in the
		// future to do things like:
		//    a) Allow things like underline/blink/inverse/crossed-out/...
		//    b) Allow the standard 8 ASNI colors
		//    c) Allow the standard 16 ASNI colors
		//    d) Use terminfo to decide what to let through.

		// Text Style and Appearance
		//    0: Reset/Normal (resets all attributes)
		//    1: Bold or increased intensity
		//    2: Faint (decreased intensity)
		//    3: Italic
		//    4: Underline
		//    5: Slow Blink
		//    6: Rapid Blink
		//    7: Reverse/Invert (foreground and background colors swapped)
		//    8: Conceal/Hidden
		//    9: Crossed-out
		//    21: Bold off or Double Underline
		//    22: Normal color or intensity (neither bold nor faint)
		//    23: Not italicized
		//    24: Underline off
		//    25: Blink off
		//    27: Inverse off
		//    28: Reveal
		//    29: Not crossed-out

		// Foreground Colors (30-37, 90-97)
		//    30: Black
		//    31: Red
		//    32: Green
		//    33: Yellow
		//    34: Blue
		//    35: Magenta
		//    36: Cyan
		//    37: White
		//    39: Default (foreground color)
		//    90: Bright Black (Gray)
		//    91: Bright Red
		//    92: Bright Green
		//    93: Bright Yellow
		//    94: Bright Blue
		//    95: Bright Magenta
		//    96: Bright Cyan
		//    97: Bright White

		// Background Colors (40-47, 100-107)
		//    40: Black Background
		//    41: Red Background
		//    42: Green Background
		//    43: Yellow Background
		//    44: Blue Background
		//    45: Magenta Background
		//    46: Cyan Background
		//    47: White Background
		//    49: Default Background Color
		//    100: Bright Black Background
		//    101: Bright Red Background
		//    102: Bright Green Background
		//    103: Bright Yellow Background
		//    104: Bright Blue Background
		//    105: Bright Magenta Background
		//    106: Bright Cyan Background
		//    107: Bright White Background
		//
		// Extended Colors (256 Colors)
		//    Foreground: ESC[38;5;<n>m (0-255)
		//    Background: ESC[48;5;<n>m (0-255)
		//
		// Extended Colors (True Color / RGB)
		//    Foreground: ESC[38;2;<r>;<g>;<b>m
		//    Background: ESC[48;2;<r>;<g>;<b>m
		return true
	}
	if s.Code == ansi.OSC && len(s.Params) > 0 {
		// These are probably harmless but they will not
		// provide any benefit to the application.
		switch s.Params[0] {
		case "4": // Query color palette
		case "10": // Query foreground color
		case "11": // Query background color
		case "12": // Query cursor color
			return true
		}
	}
	return false
}

// saved is used for partial escape sequences received
var saved []byte

func filter(buf []byte, w io.Writer) {
	buf = append(saved, buf...)
	saved = nil
	for len(buf) > 0 {
		left, s, err := ansi.Decode(buf)
		if err == ansi.IncompleteCSI || err == ansi.NoST {
			saved = append([]byte(nil), buf...)
			return
		}
		used := buf[:len(buf)-len(left)] // What we consumed
		buf = left                       // What was left
		if strip(s) {
			continue
		}

		w.Write(used)
		if !debug || s.Type == "" {
			continue
		}

		// Everything below is just for debugging what escape sequences
		// a program is generating.
		fmt.Fprint(log, "\n")
		fmt.Fprintf(log, "Input: %q\n", used)
		if err != nil {
			fmt.Fprintf(log, "Error: %v\n", err)
		}
		fmt.Fprintf(log, "Code: %q\n", s.Code)
		fmt.Fprintf(log, "Type: %q\n", s.Type)
		if len(s.Params) > 0 {
			fmt.Fprintf(log, "Params: %q\n", s.Params)
		}
		if seq := s.Code.S(); seq != nil {
			fmt.Fprintf(log, "Desc: %s\n", seq.Desc)
		}
	}
}
