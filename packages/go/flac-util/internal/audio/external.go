package audio

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
)

// FileHash returns the SHA-256 of a whole file.
//
// The MD5 in a FLAC header covers the decoded audio, so two files with the same
// audio and different tags share it. This covers the bytes on disk, which is
// what finds the same file twice in a library rather than the same recording.
func FileHash(path string, bufferSize int) (string, error) {
	f, err := os.Open(path) //nolint:gosec // the path is the file the caller asked to measure
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // opened read-only

	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}

	digest := sha256.New()
	if _, err := io.Copy(digest, bufio.NewReaderSize(f, bufferSize)); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// VerifyResult is what the reference decoder made of a file.
type VerifyResult struct {
	// Ran is false when the tool was not available, which is not a fault in
	// the file.
	Ran bool
	// OK is true when the reference decoder read the file without complaint.
	OK bool
	// Output is what it said, kept whether or not it succeeded.
	Output string
}

// Verify runs the reference decoder over a file.
//
// It is a second opinion on the same question this program's own decoder
// answers, from the implementation everything else in the world is checked
// against. Agreement is worth recording; disagreement is worth a great deal
// more, and neither is knowable without asking.
func Verify(ctx context.Context, tool, path string) (VerifyResult, error) {
	if tool == "" {
		tool = "flac"
	}
	binary, err := exec.LookPath(tool)
	if errors.Is(err, exec.ErrNotFound) {
		return VerifyResult{}, nil
	}
	if err != nil {
		return VerifyResult{}, err
	}

	//nolint:gosec // consulting the reference decoder is the point of this call
	cmd := exec.CommandContext(ctx, binary, "--test", "--silent", path)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out

	err = cmd.Run()
	result := VerifyResult{Ran: true, OK: err == nil, Output: strings.TrimSpace(out.String())}

	var exit *exec.ExitError
	if err == nil || errors.As(err, &exit) {
		// A non-zero exit is the tool's verdict, not a failure to consult it.
		return result, nil
	}
	return result, err
}

// FingerprintResult is an acoustic fingerprint of a file's audio.
type FingerprintResult struct {
	// Ran is false when the tool was not available.
	Ran bool
	// Fingerprint is the compressed AcoustID fingerprint.
	Fingerprint string
	// Duration is the length the tool measured, in seconds.
	Duration string
}

// Fingerprint runs chromaprint over a file.
//
// The fingerprint identifies the recording rather than the file, so two rips
// of the same disc share it however differently they were encoded or tagged.
// That is what makes it possible to line up a library against itself, and
// against MusicBrainz, without trusting anybody's metadata.
func Fingerprint(ctx context.Context, tool, path string) (FingerprintResult, error) {
	if tool == "" {
		tool = "fpcalc"
	}
	binary, err := exec.LookPath(tool)
	if errors.Is(err, exec.ErrNotFound) {
		return FingerprintResult{}, nil
	}
	if err != nil {
		return FingerprintResult{}, err
	}

	//nolint:gosec // running the fingerprinter is the point of this call
	cmd := exec.CommandContext(ctx, binary, path)
	out, err := cmd.Output()
	if err != nil {
		return FingerprintResult{Ran: true}, err
	}

	result := FingerprintResult{Ran: true}
	for line := range strings.SplitSeq(string(out), "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}
		switch key {
		case "FINGERPRINT":
			result.Fingerprint = value
		case "DURATION":
			result.Duration = value
		}
	}
	return result, nil
}
