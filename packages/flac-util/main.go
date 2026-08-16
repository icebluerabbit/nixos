// Command flac-util measures FLAC files and records what it found.
//
// It writes observations, never conclusions: nothing it produces says whether
// a file is genuine, because that judgement belongs to whoever reads the
// numbers. Audio files are opened read-only and are never modified.
//
// main is deliberately thin. See the cmd package for the interface and the
// internal packages for the work.
package main

import (
	"os"

	"flac-util/cmd"
)

func main() {
	os.Exit(cmd.Execute())
}
