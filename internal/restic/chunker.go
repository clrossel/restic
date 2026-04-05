package restic

import (
	"io"

	"github.com/kalbasit/fastcdc"
	rabincdc "github.com/restic/chunker"
	"github.com/restic/restic/internal/errors"
)

type ChunkerAlgorithm string

const (
	ChunkerAlgorithmRabin       ChunkerAlgorithm = "rabin"
	ChunkerAlgorithmFastCDC     ChunkerAlgorithm = "fastcdc"
	ChunkerAlgorithmUnsupported ChunkerAlgorithm = "unsupported"
)

type Chunk struct {
	Start       uint64
	Length      uint
	Fingerprint uint64
	Data        []byte
}

type Chunker interface {
	Next([]byte) (Chunk, error)
	Reset(io.Reader, Config)
}

type RabinChunker struct {
	chunker *rabincdc.Chunker
}

func (s RabinChunker) Reset(reader io.Reader, config Config) {
	s.chunker.Reset(reader, config.ChunkerPolynomial)
}

func (s RabinChunker) Next(buffer []byte) (Chunk, error) {
	c, err := s.chunker.Next(buffer)

	if err != nil {
		return Chunk{}, err
	}

	return Chunk{
		Start:       uint64(c.Start),
		Length:      c.Length,
		Fingerprint: c.Cut,
		Data:        c.Data,
	}, nil
}

type FastCdcChunker struct {
	chunker *fastcdc.Chunker
}

func (s FastCdcChunker) Reset(reader io.Reader, _ Config) {
	s.chunker.Reset(reader)
}

func (s FastCdcChunker) Next(buffer []byte) (Chunk, error) {
	c, err := s.chunker.Next()

	if err != nil {
		return Chunk{}, err
	}

	return Chunk{
		Start:       c.Offset,
		Length:      uint(c.Length),
		Fingerprint: c.Hash,
		Data:        append(buffer[:0], c.Data...),
	}, nil
}

func NewChunker(r io.Reader, config Config) (Chunker, error) {
	switch config.ChunkerAlgorithm {
	case ChunkerAlgorithmRabin:
		ch := rabincdc.NewWithBoundaries(r, config.ChunkerPolynomial, rabincdc.MinSize, rabincdc.MaxSize)
		return RabinChunker{
			chunker: ch,
		}, nil

	case ChunkerAlgorithmFastCDC:
		ch, err := fastcdc.NewChunker(r,
			// Use the same parameters as the rolling Rabin hash (for now).
			fastcdc.WithMinSize(rabincdc.MinSize), fastcdc.WithMaxSize(rabincdc.MaxSize),
			fastcdc.WithTargetSize(1024*1024),
		)
		if err != nil {
			return nil, err
		}
		return FastCdcChunker{
			chunker: ch,
		}, nil

	default:
		return nil, errors.Errorf("unsupported chunker algorithm %q", config.ChunkerAlgorithm)
	}
}
