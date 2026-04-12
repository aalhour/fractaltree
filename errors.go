package fractaltree

import "errors"

// Sentinel errors returned by the library.
var (
	// ErrClosed is returned when an operation is attempted on a closed tree.
	ErrClosed = errors.New("fractaltree: tree is closed")

	// ErrInvalidEpsilon is returned when epsilon is not in the valid range (0, 1].
	ErrInvalidEpsilon = errors.New("fractaltree: epsilon must be in (0, 1]")

	// ErrNilCompare is returned when a nil comparator is passed to NewWithCompare.
	ErrNilCompare = errors.New("fractaltree: compare function must not be nil")

	// ErrInvalidBlockSize is returned when block size is less than 2.
	ErrInvalidBlockSize = errors.New("fractaltree: block size must be at least 2")
)
