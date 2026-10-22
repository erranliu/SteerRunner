package parsers

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"

	"github.com/klauspost/compress/zstd"
)

var errInvalidFrame = errors.New("invalid zstd frame")
var errDecodeBudget = errors.New("decoded budget exceeded")

// verifiedFrames exposes bytes only after a whole frame, including its checksum,
// has decoded successfully. A truncated final frame cannot leak apparently
// complete JSON records into the trustworthy prefix.
type verifiedFrames struct {
	ctx             context.Context
	source, pending []byte
	decoder         *zstd.Decoder
	total           int
}

func (r *verifiedFrames) Read(dst []byte) (int, error) {
	for len(r.pending) == 0 {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
		if len(r.source) == 0 {
			return 0, io.EOF
		}
		length, err := frameLength(r.source)
		if err != nil {
			return 0, err
		}
		var header zstd.Header
		_ = header.Decode(r.source)
		if header.HasFCS && header.FrameContentSize > uint64(MaxDecoded-r.total) || header.WindowSize > MaxSource {
			return 0, errDecodeBudget
		}
		if err = r.decoder.Reset(bytes.NewReader(r.source[:length])); err != nil {
			return 0, errors.Join(errInvalidFrame, err)
		}
		decoded, err := io.ReadAll(io.LimitReader(r.decoder, int64(MaxDecoded-r.total)+1))
		if err != nil {
			return 0, errors.Join(errInvalidFrame, err)
		}
		if len(decoded) > MaxDecoded-r.total {
			return 0, errDecodeBudget
		}
		r.total += len(decoded)
		r.pending = decoded
		r.source = r.source[length:]
	}
	n := copy(dst, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}
func frameLength(raw []byte) (int, error) {
	var header zstd.Header
	if err := header.Decode(raw); err != nil {
		return 0, err
	}
	offset := header.HeaderSize
	if header.Skippable {
		if uint64(offset)+uint64(header.SkippableSize) > uint64(len(raw)) {
			return 0, io.ErrUnexpectedEOF
		}
		return offset + int(header.SkippableSize), nil
	}
	for {
		if len(raw)-offset < 3 {
			return 0, io.ErrUnexpectedEOF
		}
		var word [4]byte
		copy(word[:], raw[offset:offset+3])
		block := binary.LittleEndian.Uint32(word[:])
		offset += 3
		blockType := (block >> 1) & 3
		size := int(block >> 3)
		if blockType == 3 {
			return 0, errInvalidFrame
		}
		if blockType == 1 {
			size = 1
		}
		if size > len(raw)-offset {
			return 0, io.ErrUnexpectedEOF
		}
		offset += size
		if block&1 != 0 {
			break
		}
	}
	if header.HasCheckSum {
		if len(raw)-offset < 4 {
			return 0, io.ErrUnexpectedEOF
		}
		offset += 4
	}
	return offset, nil
}
