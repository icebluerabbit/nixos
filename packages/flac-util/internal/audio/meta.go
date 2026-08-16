// Package audio reads FLAC files: the metadata blocks that describe them, and
// the audio samples streamed through a set of measurements.
//
// Files are opened read-only and are never modified. Callers are expected to
// be pointing at a library whose files may simultaneously be seeded by a
// torrent client, where a single byte written in the wrong place has a real
// cost.
package audio

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/meta"

	"flac-util/internal/measure"
)

// Names of tag formats that FLAC does not define. Their presence is a fact
// about a file's history - it passed through tooling built for some other
// format - rather than a fact about its audio.
const (
	TagID3v2 = "ID3v2"
	TagID3v1 = "ID3v1"
	TagAPEv2 = "APEv2"
)

// probeBufferSize is the read-ahead used while parsing metadata blocks, which
// are small and sit at the front of the file.
const probeBufferSize = 64 << 10

// StreamInfo mirrors the mandatory first metadata block of a FLAC stream: what
// the file claims about itself.
type StreamInfo struct {
	SampleRate    int
	Channels      int
	BitsPerSample int
	// TotalSamples is the declared number of inter-channel samples. Zero means
	// the encoder did not record it.
	TotalSamples uint64
	BlockSizeMin int
	BlockSizeMax int
	// FrameSizeMin and FrameSizeMax are in bytes. Zero means unknown.
	FrameSizeMin int
	FrameSizeMax int
	// MD5 is the lowercase hex checksum of the unencoded audio, as recorded by
	// whatever encoded the file.
	MD5 string
	// MD5Present is false when the encoder left the field zeroed, which some
	// tools do. An absent checksum cannot be checked against the audio.
	MD5Present bool
}

// Format converts the header into the form the measurement package expects.
func (s StreamInfo) Format() measure.Format {
	return measure.Format{
		SampleRate:    s.SampleRate,
		Channels:      s.Channels,
		BitsPerSample: s.BitsPerSample,
	}
}

// Duration returns the length of the stream in seconds, or zero when the
// header does not record enough to work it out.
func (s StreamInfo) Duration() float64 {
	if s.SampleRate <= 0 {
		return 0
	}
	return float64(s.TotalSamples) / float64(s.SampleRate)
}

// Metadata is everything a file says about itself without decoding any audio.
type Metadata struct {
	StreamInfo StreamInfo
	// Vendor is the encoder string recorded in the Vorbis comment block.
	Vendor string
	// Tags are the name/value pairs of the Vorbis comment block, verbatim and
	// in file order.
	Tags [][2]string
	// HasVorbisComment reports whether the block was present at all, which
	// distinguishes an untagged file from one tagged with nothing.
	HasVorbisComment  bool
	SeekPoints        int
	HasCueSheet       bool
	Pictures          int
	ApplicationBlocks int
	// PaddingBytes is the total size of padding blocks.
	PaddingBytes int64
	// ForeignTags names tag formats found that FLAC does not define.
	ForeignTags []string
	// AudioOffset is where the FLAC signature begins, which is non-zero when
	// the file carries a leading tag block.
	AudioOffset int64
	// FileSize is the size of the file on disk, in bytes.
	FileSize int64
}

// Probe reads the metadata of a FLAC file without decoding its audio.
func Probe(path string) (*Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	m, err := ProbeReader(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("probe %s: %w", path, err)
	}
	return m, nil
}

// ProbeReader reads the metadata of a FLAC stream of the given size.
func ProbeReader(rs io.ReadSeeker, size int64) (*Metadata, error) {
	m := &Metadata{FileSize: size}

	offset, leading, err := leadingTagLength(rs)
	if err != nil {
		return nil, err
	}
	m.AudioOffset = offset
	m.ForeignTags = append(m.ForeignTags, leading...)

	trailing, err := trailingTagNames(rs, size)
	if err != nil {
		return nil, err
	}
	m.ForeignTags = append(m.ForeignTags, trailing...)

	if _, err := rs.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	stream, err := flac.Parse(bufio.NewReaderSize(rs, probeBufferSize))
	if err != nil {
		return nil, fmt.Errorf("parse metadata: %w", err)
	}

	m.StreamInfo = convertStreamInfo(stream.Info)
	applyBlocks(m, stream.Blocks)
	return m, nil
}

// convertStreamInfo translates the decoder's header into ours.
func convertStreamInfo(info *meta.StreamInfo) StreamInfo {
	out := StreamInfo{
		SampleRate:    int(info.SampleRate),
		Channels:      int(info.NChannels),
		BitsPerSample: int(info.BitsPerSample),
		TotalSamples:  info.NSamples,
		BlockSizeMin:  int(info.BlockSizeMin),
		BlockSizeMax:  int(info.BlockSizeMax),
		FrameSizeMin:  int(info.FrameSizeMin),
		FrameSizeMax:  int(info.FrameSizeMax),
	}

	var digits strings.Builder
	for _, b := range info.MD5sum {
		if b != 0 {
			out.MD5Present = true
		}
		fmt.Fprintf(&digits, "%02x", b)
	}
	if out.MD5Present {
		out.MD5 = digits.String()
	}
	return out
}

// applyBlocks folds the optional metadata blocks into m.
func applyBlocks(m *Metadata, blocks []*meta.Block) {
	for _, block := range blocks {
		switch body := block.Body.(type) {
		case *meta.VorbisComment:
			m.HasVorbisComment = true
			m.Vendor = body.Vendor
			m.Tags = append(m.Tags, body.Tags...)
		case *meta.SeekTable:
			m.SeekPoints += len(body.Points)
		case *meta.CueSheet:
			m.HasCueSheet = true
		case *meta.Picture:
			m.Pictures++
		case *meta.Application:
			m.ApplicationBlocks++
		default:
			if block.Type == meta.TypePadding {
				m.PaddingBytes += block.Length
			}
		}
	}
}

// leadingTagLength reports how many bytes precede the FLAC signature, along
// with the names of any foreign tag formats found there.
//
// An ID3v2 block prepended to a FLAC file is malformed but common, and the
// decoder cannot read past it on its own.
func leadingTagLength(rs io.ReadSeeker) (int64, []string, error) {
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return 0, nil, err
	}

	header := make([]byte, 10)
	if _, err := io.ReadFull(rs, header); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return 0, nil, nil
		}
		return 0, nil, err
	}
	if string(header[:3]) != "ID3" {
		return 0, nil, nil
	}

	// The four size bytes are "syncsafe": seven significant bits each, so that
	// the encoded length can never look like a frame sync.
	var size int64
	for _, b := range header[6:10] {
		if b&0x80 != 0 {
			return 0, nil, fmt.Errorf("malformed ID3v2 size byte %#02x", b)
		}
		size = size<<7 | int64(b&0x7f)
	}

	length := 10 + size
	const footerPresent = 0x10
	if header[5]&footerPresent != 0 {
		length += 10
	}
	return length, []string{TagID3v2}, nil
}

// trailingTagNames reports the foreign tag formats appended to the file.
func trailingTagNames(rs io.ReadSeeker, size int64) ([]string, error) {
	const (
		id3v1Length = 128
		apeLength   = 32
	)

	var names []string
	hasID3v1, err := hasMarkerAt(rs, size-id3v1Length, []byte("TAG"))
	if err != nil {
		return nil, err
	}
	if hasID3v1 {
		names = append(names, TagID3v1)
	}

	// An APEv2 footer sits at the very end, or just before an ID3v1 block when
	// both are present.
	offsets := []int64{size - apeLength}
	if hasID3v1 {
		offsets = append(offsets, size-id3v1Length-apeLength)
	}
	for _, offset := range offsets {
		found, err := hasMarkerAt(rs, offset, []byte("APETAGEX"))
		if err != nil {
			return nil, err
		}
		if found {
			names = append(names, TagAPEv2)
			break
		}
	}
	return names, nil
}

// hasMarkerAt reports whether the given bytes appear at the given offset.
func hasMarkerAt(rs io.ReadSeeker, offset int64, marker []byte) (bool, error) {
	if offset < 0 {
		return false, nil
	}
	if _, err := rs.Seek(offset, io.SeekStart); err != nil {
		return false, err
	}

	buf := make([]byte, len(marker))
	if _, err := io.ReadFull(rs, buf); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return false, nil
		}
		return false, err
	}
	return string(buf) == string(marker), nil
}
