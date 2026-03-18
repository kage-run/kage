package sandbox

import (
	"bytes"
	"context"
	"sync"
)

// RingBuffer is a thread-safe circular buffer that stores the last N lines of output.
// It implements io.Writer so it can be used directly with exec.Cmd stdout/stderr.
type RingBuffer struct {
	mu      sync.Mutex
	lines   []string
	cap     int
	pos     int // next write position (circular)
	count   int // total lines ever written
	partial bytes.Buffer
	cond    *sync.Cond
}

// NewRingBuffer creates a ring buffer that holds up to capacity lines.
func NewRingBuffer(capacity int) *RingBuffer {
	rb := &RingBuffer{
		lines: make([]string, capacity),
		cap:   capacity,
	}
	rb.cond = sync.NewCond(&rb.mu)
	return rb
}

// Write implements io.Writer. It splits input on newlines and stores each line.
// Partial lines (without trailing newline) are buffered until the next Write.
func (rb *RingBuffer) Write(p []byte) (n int, err error) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	n = len(p)
	rb.partial.Write(p)

	for {
		line, err := rb.partial.ReadBytes('\n')
		if err != nil {
			// No more complete lines; put the partial back
			rb.partial.Write(line)
			break
		}
		// Store the line without the trailing newline
		s := string(bytes.TrimRight(line, "\n"))
		rb.lines[rb.pos] = s
		rb.pos = (rb.pos + 1) % rb.cap
		rb.count++
		rb.cond.Broadcast()
	}

	return n, nil
}

// Lines returns the last n lines from the buffer.
// If fewer than n lines exist, returns all available lines.
func (rb *RingBuffer) Lines(n int) []string {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	return rb.linesLocked(n)
}

func (rb *RingBuffer) linesLocked(n int) []string {
	available := rb.count
	if available > rb.cap {
		available = rb.cap
	}
	if n > available {
		n = available
	}
	if n == 0 {
		return nil
	}

	result := make([]string, n)
	start := (rb.pos - n + rb.cap) % rb.cap
	for i := 0; i < n; i++ {
		result[i] = rb.lines[(start+i)%rb.cap]
	}
	return result
}

// Flush returns any partial (unterminated) line currently buffered.
func (rb *RingBuffer) Flush() string {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.partial.String()
}

// Follow returns a channel that emits new lines as they are written.
// The channel closes when the context is canceled.
func (rb *RingBuffer) Follow(ctx context.Context) <-chan string {
	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		rb.mu.Lock()
		lastSeen := rb.count
		rb.mu.Unlock()

		for {
			rb.mu.Lock()
			for rb.count == lastSeen {
				// Wait for new lines or context cancellation
				done := make(chan struct{})
				go func() {
					select {
					case <-ctx.Done():
						rb.cond.Broadcast()
					case <-done:
					}
				}()
				rb.cond.Wait()
				close(done)

				if ctx.Err() != nil {
					rb.mu.Unlock()
					return
				}
			}

			// Collect new lines
			newCount := rb.count - lastSeen
			if newCount > rb.cap {
				newCount = rb.cap
			}
			newLines := rb.linesLocked(newCount)
			lastSeen = rb.count
			rb.mu.Unlock()

			for _, line := range newLines {
				select {
				case ch <- line:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return ch
}
