package restic

import (
	"context"
	"sync"
	"testing"

	"github.com/restic/restic/internal/errors"

	"github.com/restic/restic/internal/debug"

	"github.com/restic/chunker"
)

// Config contains the configuration for a repository.
type Config struct {
	Version           uint             `json:"version"`
	ID                string           `json:"id"`
	ChunkerPolynomial chunker.Pol      `json:"chunker_polynomial,omitempty"`
	ChunkerAlgorithm  ChunkerAlgorithm `json:"chunker_algorithm,omitempty"`
}

const MinRepoVersion = 1
const MaxRepoVersion = 3

// StableRepoVersion is the version that is written to the config when a repository
// is newly created with Init().
const StableRepoVersion = MaxRepoVersion

// JSONUnpackedLoader loads unpacked JSON.
type JSONUnpackedLoader interface {
	LoadJSONUnpacked(context.Context, FileType, ID, interface{}) error
}

func defaultChunkerAlgorithmForVersion(version uint) ChunkerAlgorithm {
	switch {
	case version <= 2:
		return ChunkerAlgorithmRabin
	case version == 3:
		return ChunkerAlgorithmFastCDC
	default:
		return ChunkerAlgorithmUnsupported
	}
}

// CreateConfig creates a config file with a randomly selected polynomial and
// ID.
func CreateConfig(version uint) (Config, error) {
	var (
		err error
		cfg Config
	)

	algorithm := defaultChunkerAlgorithmForVersion(version)

	if algorithm == ChunkerAlgorithmUnsupported {
		return Config{}, errors.New("unsupported chunker algorithm")
	} else if algorithm == ChunkerAlgorithmRabin {
		cfg.ChunkerPolynomial, err = chunker.RandomPolynomial()
		if err != nil {
			return Config{}, errors.Wrap(err, "chunker.RandomPolynomial")
		}
	}

	cfg.ID = NewRandomID().String()
	cfg.Version = version

	debug.Log("New config: %#v", cfg)
	return cfg, nil
}

var checkPolynomial = true
var checkPolynomialOnce sync.Once

// TestDisableCheckPolynomial disables the check that the polynomial used for
// the chunker.
func TestDisableCheckPolynomial(t testing.TB) {
	t.Logf("disabling check of the chunker polynomial")
	checkPolynomialOnce.Do(func() {
		checkPolynomial = false
	})
}

// LoadConfig returns loads, checks and returns the config for a repository.
func LoadConfig(ctx context.Context, r LoaderUnpacked) (Config, error) {
	var (
		cfg Config
	)

	err := LoadJSONUnpacked(ctx, r, ConfigFile, ID{}, &cfg)
	if err != nil {
		return Config{}, err
	}

	if cfg.Version < MinRepoVersion || cfg.Version > MaxRepoVersion {
		return Config{}, errors.Errorf("unsupported repository version %v", cfg.Version)
	}

	algorithm := defaultChunkerAlgorithmForVersion(cfg.Version)

	// Backfill algorithm for old repos that won't have new field
	if cfg.ChunkerAlgorithm == "" {
		cfg.ChunkerAlgorithm = algorithm
	}

	switch algorithm {
	case ChunkerAlgorithmRabin:
		if checkPolynomial && !cfg.ChunkerPolynomial.Irreducible() {
			return Config{}, errors.New("invalid chunker polynomial")
		}
	case ChunkerAlgorithmFastCDC:
		// no polynomial validation
	default:
		return Config{}, errors.New("unsupported chunker algorithm")
	}

	return cfg, nil
}

func SaveConfig(ctx context.Context, r SaverUnpacked[FileType], cfg Config) error {
	_, err := SaveJSONUnpacked(ctx, r, ConfigFile, cfg)
	return err
}
