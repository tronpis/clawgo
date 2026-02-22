package audio

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLineCapturePathReopenStreamsMultipleLines(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "input.txt")
	if err := os.WriteFile(path, []byte("first\n"), 0o600); err != nil {
		t.Fatalf("write first line: %v", err)
	}

	capture := NewLineCaptureFromPath(path, t.Logf)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	frames, err := capture.Start(ctx)
	if err != nil {
		t.Fatalf("start capture: %v", err)
	}

	deadline := time.After(3 * time.Second)
	seenFirst := false
	seenSecond := false
	for !seenSecond {
		select {
		case frame, ok := <-frames:
			if !ok {
				t.Fatalf("capture channel closed early; seenFirst=%v", seenFirst)
			}
			text := string(frame.Data)
			switch text {
			case "first":
				if !seenFirst {
					seenFirst = true
					if err := os.WriteFile(path, []byte("second\n"), 0o600); err != nil {
						t.Fatalf("write second line: %v", err)
					}
				}
			case "second":
				seenSecond = true
			default:
				t.Fatalf("unexpected frame: %q", text)
			}
		case <-deadline:
			t.Fatalf("timeout waiting for second frame; seenFirst=%v", seenFirst)
		}
	}

	if !seenFirst {
		t.Fatalf("did not observe first frame before second")
	}
}

func TestLineCapturePathDoesNotReplayUnchangedFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "input.txt")
	if err := os.WriteFile(path, []byte("only-once\n"), 0o600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	capture := NewLineCaptureFromPath(path, t.Logf)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	frames, err := capture.Start(ctx)
	if err != nil {
		t.Fatalf("start capture: %v", err)
	}

	select {
	case frame, ok := <-frames:
		if !ok {
			t.Fatalf("capture channel closed before first frame")
		}
		if got := string(frame.Data); got != "only-once" {
			t.Fatalf("unexpected first frame: %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for first frame")
	}

	select {
	case frame := <-frames:
		t.Fatalf("unexpected duplicate frame from unchanged file: %q", string(frame.Data))
	case <-time.After(400 * time.Millisecond):
		// pass: no replay for unchanged file
	}
}
