package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ipfs/go-cid"
)

// ProducerID is a unique identifier for a tuple producer.
// It is a string to allow for descriptive, custom IDs.
type ProducerID string

// Tuple represents the data structure emitted by producers.
type Tuple struct {
	Slot       uint64
	Hash       cid.Cid
	Value      any
	ProducerID ProducerID
}

// MismatchCallback is the function signature for the callback that is executed
// when a set of tuples with the same ID have different hashes.
type MismatchCallback func(tuples []Tuple)

// Emitter is a function returned to a registered producer.
// Calling this function sends a tuple to the matcher.
type Emitter func(id uint64, hash cid.Cid, value any)

// Matcher orchestrates the process of receiving tuples from multiple producers,
// pairing them by ID, and checking for hash mismatches.
type Matcher struct {
	inChan       chan Tuple
	pending      map[uint64][]Tuple // Simplified storage
	backlogLimit int
	mismatchCb   MismatchCallback
	numProducers int
	ctx          context.Context    // Context for cancellation
	cancel       context.CancelFunc // Cancel function for the context

	numCompared uint64 // Counter for the number of compared tuples
	numMatches  uint64 // Counter for the number of matching tuples
	numDiffers  uint64 // Counter for the number of differing tuples

	// For registration and graceful shutdown
	regMutex            sync.Mutex
	registeredProducers map[ProducerID]struct{}
	wg                  sync.WaitGroup
	shutdown            chan struct{}
}

// NewMatcher creates and initializes a new Matcher.
func NewMatcher(
	ctx context.Context, // Context for cancellation
	numProducers, backlogLimit int, cb MismatchCallback,
) *Matcher {
	if backlogLimit <= 0 {
		backlogLimit = 1
	}
	if numProducers < 2 {
		numProducers = 2
	}
	ctx, cancel := context.WithCancel(ctx)
	return &Matcher{
		ctx:                 ctx,
		cancel:              cancel,
		inChan:              make(chan Tuple),
		pending:             make(map[uint64][]Tuple),
		backlogLimit:        backlogLimit,
		mismatchCb:          cb,
		numProducers:        numProducers,
		registeredProducers: make(map[ProducerID]struct{}),
		shutdown:            make(chan struct{}),
	}
}

type Counts struct {
	Compared uint64
	Matches  uint64
	Differs  uint64
	Pending  int
}

func (m *Matcher) GetCounts() Counts {
	m.regMutex.Lock()
	defer m.regMutex.Unlock()

	return Counts{
		Compared: atomic.LoadUint64(&m.numCompared),
		Matches:  atomic.LoadUint64(&m.numMatches),
		Differs:  atomic.LoadUint64(&m.numDiffers),
		Pending:  len(m.pending),
	}
}

// RegisterProducer validates a producer's custom ID and returns a dedicated
// Emitter function for that producer to send tuples.
func (m *Matcher) RegisterProducer(id ProducerID) (Emitter, error) {
	m.regMutex.Lock()
	defer m.regMutex.Unlock()

	if id == "" {
		return nil, fmt.Errorf("producer ID cannot be empty")
	}
	if _, exists := m.registeredProducers[id]; exists {
		return nil, fmt.Errorf("producer with ID '%s' is already registered", id)
	}

	m.registeredProducers[id] = struct{}{}

	// Return a closure that acts as the emitter for this specific producer.
	emitter := func(tupleID uint64, hash cid.Cid, value any) {
		tuple := Tuple{
			Slot:       tupleID,
			Hash:       hash,
			Value:      value,
			ProducerID: id, // The producer's ID is captured by the closure.
		}
		// This might block if the inChan is full, providing natural backpressure.
		m.inChan <- tuple
	}

	return emitter, nil
}

// Start begins the listening process in a background goroutine.
// It now only returns an error channel, as producers use their Emitter func.
func (m *Matcher) Start() <-chan error {
	errChan := make(chan error, 1)

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		// fmt.Println("Matcher started. Waiting for tuples...")
		for {
			select {
			case <-m.ctx.Done():
				// Context cancellation, exit the loop.
				// fmt.Println("Matcher context cancelled. Shutting down.")
				close(errChan)
				return
			case tuple := <-m.inChan:
				// Append the new tuple to the slice for its ID.
				m.pending[tuple.Slot] = append(m.pending[tuple.Slot], tuple)

				// Check if we have received a tuple from all producers for this ID.
				if len(m.pending[tuple.Slot]) == m.numProducers {
					// fmt.Printf("Matcher: Completed set for ID %d.\n", tuple.ID)
					atomic.AddUint64(&m.numCompared, 1)

					completedSet := m.pending[tuple.Slot]

					// Compare hashes.
					referenceHash := completedSet[0].Hash
					mismatchFound := false
					for i := 1; i < len(completedSet); i++ {
						if !referenceHash.Equals(completedSet[i].Hash) {
							mismatchFound = true
							break
						}
					}

					if mismatchFound {
						atomic.AddUint64(&m.numDiffers, 1)
						// fmt.Printf("Matcher: Hash mismatch for ID %d! Triggering callback.\n", tuple.ID)
						m.mismatchCb(completedSet)
					} else {
						atomic.AddUint64(&m.numMatches, 1)
						// fmt.Printf("Matcher: All hashes match for ID %d.\n", tuple.ID)
					}

					// The set is processed, remove it from the pending map.
					delete(m.pending, tuple.Slot)
				}

				// Check for backlog limit violation *after* processing a potential completion.
				// This prevents a backlog error on the very item that would clear another.
				if len(m.pending) >= m.backlogLimit && len(m.pending[tuple.Slot]) < m.numProducers {
					err := fmt.Errorf("backlog limit of %d exceeded. current size: %d; THERE HAVEN'T BEEN A SINGLE MATCHING BLOCK FOR TOO LONG", m.backlogLimit, len(m.pending))
					// fmt.Printf("Matcher: ERROR - %s\n", err.Error())
					errChan <- err
					<-m.shutdown // Wait for shutdown signal
					return
				}

			case <-m.shutdown:
				// fmt.Println("Matcher shutting down.")
				close(errChan)
				return
			}
		}
	}()

	return errChan
}

// Stop gracefully shuts down the matcher and waits for its goroutine to finish.
func (m *Matcher) Stop() {
	m.regMutex.Lock()
	defer m.regMutex.Unlock()

	if m.shutdown != nil {
		close(m.shutdown) // Signal the matcher to stop
		m.wg.Wait()       // Wait for the matcher goroutine to finish
		m.shutdown = nil  // Prevent further shutdown calls
	}

	if m.cancel != nil {
		m.cancel() // Cancel the context to stop any ongoing operations
		m.cancel = nil
	}
}
