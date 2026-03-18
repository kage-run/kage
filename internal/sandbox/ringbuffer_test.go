package sandbox

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRingBuffer_BasicWriteRead(t *testing.T) {
	rb := NewRingBuffer(10)
	fmt.Fprintln(rb, "hello")
	fmt.Fprintln(rb, "world")

	lines := rb.Lines(10)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestRingBuffer_Overflow(t *testing.T) {
	rb := NewRingBuffer(3)
	for i := 0; i < 5; i++ {
		fmt.Fprintf(rb, "line %d\n", i)
	}

	lines := rb.Lines(10)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line 2" || lines[1] != "line 3" || lines[2] != "line 4" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestRingBuffer_PartialLines(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Write([]byte("hel"))
	rb.Write([]byte("lo\nwor"))
	rb.Write([]byte("ld\n"))

	lines := rb.Lines(10)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "hello" || lines[1] != "world" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestRingBuffer_LinesRequestFewer(t *testing.T) {
	rb := NewRingBuffer(10)
	for i := 0; i < 5; i++ {
		fmt.Fprintf(rb, "line %d\n", i)
	}

	lines := rb.Lines(2)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "line 3" || lines[1] != "line 4" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestRingBuffer_Empty(t *testing.T) {
	rb := NewRingBuffer(10)
	lines := rb.Lines(5)
	if lines != nil {
		t.Fatalf("expected nil, got %v", lines)
	}
}

func TestRingBuffer_ConcurrentWrites(t *testing.T) {
	rb := NewRingBuffer(1000)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				fmt.Fprintf(rb, "goroutine %d line %d\n", id, j)
			}
		}(i)
	}
	wg.Wait()

	lines := rb.Lines(1000)
	if len(lines) != 1000 {
		t.Fatalf("expected 1000 lines, got %d", len(lines))
	}
}

func TestRingBuffer_Follow(t *testing.T) {
	rb := NewRingBuffer(10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := rb.Follow(ctx)

	// Write after follow starts
	time.Sleep(10 * time.Millisecond)
	fmt.Fprintln(rb, "hello")
	fmt.Fprintln(rb, "world")

	got := make([]string, 0, 2)
	timeout := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case line := <-ch:
			got = append(got, line)
		case <-timeout:
			t.Fatalf("timed out waiting for lines, got %v", got)
		}
	}

	if got[0] != "hello" || got[1] != "world" {
		t.Fatalf("unexpected follow output: %v", got)
	}

	// Cancel and verify channel closes
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			// Drain remaining
			for range ch {
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not close after cancel")
	}
}
