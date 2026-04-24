package fractaltree

import "math"

const (
	// DefaultEpsilon controls the buffer-vs-fanout trade-off.
	// At 0.5, an internal node has sqrt(B) children and sqrt(B) buffer slots.
	DefaultEpsilon = 0.5

	// DefaultBlockSize is the number of entries per node (leaf capacity).
	// With DefaultEpsilon, this yields fanout=64 and buffer capacity=64.
	DefaultBlockSize = 4096

	// minFanout is the minimum allowed fanout for an internal node.
	minFanout = 2

	// minBufferCap is the minimum allowed buffer capacity.
	minBufferCap = 1
)

// Option configures a BETree.
type Option func(*options)

type options struct {
	epsilon   float64
	blockSize int
}

func defaultOptions() options {
	return options{
		epsilon:   DefaultEpsilon,
		blockSize: DefaultBlockSize,
	}
}

func resolveOptions(opts []Option) options {
	o := defaultOptions()
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// WithEpsilon sets the epsilon parameter that controls the trade-off between
// write throughput and read latency. Must be in (0, 1].
//
// Lower epsilon means larger buffers (better write amortization) but higher
// fanout cost. Higher epsilon means smaller buffers but faster point queries.
func WithEpsilon(eps float64) Option {
	return func(o *options) {
		o.epsilon = eps
	}
}

// WithBlockSize sets the logical node size measured in number of entries.
// Must be at least 2. Default is 4096.
func WithBlockSize(size int) Option {
	return func(o *options) {
		o.blockSize = size
	}
}

// treeParams holds the derived parameters computed from options.
// These are computed once at construction and passed into node operations.
type treeParams struct {
	epsilon   float64
	blockSize int
	fanout    int // max children per internal node: B^epsilon
	bufferCap int // max messages per node buffer:   B^(1-epsilon)
	leafCap   int // max entries per leaf node:       B
}

func deriveParams(o options) treeParams {
	b := float64(o.blockSize)
	fanout := int(math.Pow(b, o.epsilon))

	fanout = max(fanout, minFanout)
	bufferCap := int(math.Pow(b, 1-o.epsilon))
	bufferCap = max(bufferCap, minBufferCap)

	return treeParams{
		epsilon:   o.epsilon,
		blockSize: o.blockSize,
		fanout:    fanout,
		bufferCap: bufferCap,
		leafCap:   o.blockSize,
	}
}

func validateOptions(o options) error {
	if o.epsilon <= 0 || o.epsilon > 1 {
		return ErrInvalidEpsilon
	}
	if o.blockSize < 2 {
		return ErrInvalidBlockSize
	}
	return nil
}
