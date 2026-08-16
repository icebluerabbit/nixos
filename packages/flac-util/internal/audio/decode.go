package audio

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/mewkiz/flac"

	"flac-util/internal/measure"
)

// DefaultBufferSize is the read-ahead applied to the underlying file.
//
// It is deliberately large: a library held on network storage answers a read
// with a round trip, and the stock buffer would turn one track into tens of
// thousands of them.
const DefaultBufferSize = 4 << 20

// DecodeOptions configures Decode.
type DecodeOptions struct {
	// Measure selects which measurements run over the audio.
	Measure measure.Options
	// BufferSize is the read-ahead in bytes. Zero selects DefaultBufferSize.
	BufferSize int
}

// DecodeResult is what one pass over a file's audio produced.
type DecodeResult struct {
	// StreamInfo is the header the decoder read, which lets a caller confirm
	// it decoded the file it probed.
	StreamInfo StreamInfo
	// Results holds the value of every measurement.
	Results measure.Results
	// Frames is the number of audio frames decoded.
	Frames int64
}

// Decode reads every sample of a FLAC file through a set of measurements.
//
// This is the expensive operation in the whole program: it is where the file
// is read off disk and where the arithmetic happens. Everything else is
// bookkeeping around it.
func Decode(ctx context.Context, path string, opts DecodeOptions) (DecodeResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return DecodeResult{}, err
	}
	defer f.Close()

	result, err := DecodeReader(ctx, f, opts)
	if err != nil {
		return result, fmt.Errorf("decode %s: %w", path, err)
	}
	return result, nil
}

// DecodeReader reads every sample of a FLAC stream through a set of
// measurements.
//
// A leading ID3v2 block needs no special handling here: the decoder skips one
// of its own accord. Probe still reports its presence, because the fact that a
// file carries one says something about where the file has been.
func DecodeReader(ctx context.Context, r io.Reader, opts DecodeOptions) (DecodeResult, error) {
	size := opts.BufferSize
	if size <= 0 {
		size = DefaultBufferSize
	}

	stream, err := flac.New(bufio.NewReaderSize(r, size))
	if err != nil {
		return DecodeResult{}, fmt.Errorf("read header: %w", err)
	}

	info := convertStreamInfo(stream.Info)
	set, err := measure.NewSet(info.Format(), opts.Measure)
	if err != nil {
		return DecodeResult{StreamInfo: info}, err
	}

	frames, err := run(ctx, stream, set, info.Channels)
	out := DecodeResult{
		StreamInfo: info,
		Results:    set.Results(),
		Frames:     frames,
	}
	return out, err
}

// run pulls frames until the stream ends, feeding each one to the accumulators.
func run(ctx context.Context, stream *flac.Stream, set *measure.Set, channels int) (int64, error) {
	// The block handed to the accumulators points straight at the decoder's
	// own sample slices; reusing the outer slice avoids an allocation per
	// frame, and accumulators are forbidden from retaining it.
	block := measure.Block{Samples: make([][]int32, 0, channels)}

	var frames int64
	for {
		if err := ctx.Err(); err != nil {
			return frames, err
		}

		frame, err := stream.ParseNext()
		if errors.Is(err, io.EOF) {
			return frames, nil
		}
		if err != nil {
			return frames, fmt.Errorf("frame %d: %w", frames, err)
		}

		block.Samples = block.Samples[:0]
		for _, subframe := range frame.Subframes {
			block.Samples = append(block.Samples, subframe.Samples)
		}
		set.Write(block)
		frames++
	}
}
