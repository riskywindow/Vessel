package progress

import (
	"bytes"
	"io"
	"testing"
)

func TestNewSpinner(t *testing.T) {
	s := NewSpinner("pulling image")
	if s == nil {
		t.Fatal("expected non-nil spinner")
	}
	if s.message != "pulling image" {
		t.Errorf("expected message 'pulling image', got %q", s.message)
	}
}

func TestSpinner_StartStop(t *testing.T) {
	s := NewSpinner("test operation")
	s.Start()
	s.Stop()
	// Should not panic or hang
}

func TestSpinner_Success(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner("deploying")
	s.writer = &buf
	s.Success("deployed v1")
	if got := buf.String(); got == "" {
		t.Error("expected success output, got empty string")
	}
}

func TestSpinner_Fail(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner("deploying")
	s.writer = &buf
	s.Fail("deploy failed")
	if got := buf.String(); got == "" {
		t.Error("expected failure output, got empty string")
	}
}

func TestSpinner_UpdateMessage(t *testing.T) {
	s := NewSpinner("initial")
	s.UpdateMessage("updated")
	if s.s.Suffix != " updated" {
		t.Errorf("expected suffix ' updated', got %q", s.s.Suffix)
	}
}

func TestNewProgressBar(t *testing.T) {
	pb := NewProgressBar(100, "downloading")
	if pb == nil {
		t.Fatal("expected non-nil progress bar")
	}
	if pb.total != 100 {
		t.Errorf("expected total 100, got %d", pb.total)
	}
	if pb.width != 40 {
		t.Errorf("expected width 40, got %d", pb.width)
	}
	if pb.message != "downloading" {
		t.Errorf("expected message 'downloading', got %q", pb.message)
	}
}

func TestProgressBar_Percent_Empty(t *testing.T) {
	pb := NewProgressBar(0, "test")
	if pct := pb.Percent(); pct != 0 {
		t.Errorf("expected 0%% for zero-total bar, got %.1f%%", pct)
	}
}

func TestProgressBar_Percent_Partial(t *testing.T) {
	pb := NewProgressBar(200, "test")
	pb.writer = io.Discard
	pb.Add(100)
	pct := pb.Percent()
	if pct < 49.9 || pct > 50.1 {
		t.Errorf("expected ~50%%, got %.1f%%", pct)
	}
}

func TestProgressBar_Percent_Full(t *testing.T) {
	pb := NewProgressBar(100, "test")
	pb.writer = io.Discard
	pb.Set(100)
	if pct := pb.Percent(); pct != 100 {
		t.Errorf("expected 100%%, got %.1f%%", pct)
	}
}

func TestProgressBar_Percent_Overflow(t *testing.T) {
	pb := NewProgressBar(100, "test")
	pb.writer = io.Discard
	pb.Set(200)
	if pct := pb.Percent(); pct != 100 {
		t.Errorf("expected capped at 100%%, got %.1f%%", pct)
	}
}

func TestProgressBar_Add(t *testing.T) {
	pb := NewProgressBar(100, "test")
	pb.writer = io.Discard
	pb.Add(25)
	pb.Add(25)
	if pb.current != 50 {
		t.Errorf("expected current 50, got %d", pb.current)
	}
}

func TestProgressBar_Set(t *testing.T) {
	pb := NewProgressBar(100, "test")
	pb.writer = io.Discard
	pb.Set(75)
	if pb.current != 75 {
		t.Errorf("expected current 75, got %d", pb.current)
	}
}

func TestProgressBar_Finish(t *testing.T) {
	var buf bytes.Buffer
	pb := NewProgressBar(100, "test")
	pb.writer = &buf
	pb.Add(50)
	pb.Finish()
	if pb.current != pb.total {
		t.Errorf("expected current == total after Finish, got %d != %d", pb.current, pb.total)
	}
}

func TestProgressBar_Draw(t *testing.T) {
	var buf bytes.Buffer
	pb := NewProgressBar(100, "downloading")
	pb.writer = &buf
	pb.Set(50)
	output := buf.String()
	if output == "" {
		t.Error("expected draw output, got empty string")
	}
}

func TestNewProgressWriter(t *testing.T) {
	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, 100, "writing")
	if pw == nil {
		t.Fatal("expected non-nil progress writer")
	}
}

func TestProgressWriter_Write(t *testing.T) {
	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, 100, "writing")
	pw.bar.writer = io.Discard

	data := []byte("hello world")
	n, err := pw.Write(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
	if buf.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", buf.String())
	}
}

func TestProgressWriter_TracksProgress(t *testing.T) {
	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, 100, "writing")
	pw.bar.writer = io.Discard

	pw.Write([]byte("12345"))
	pw.Write([]byte("67890"))

	if pw.bar.current != 10 {
		t.Errorf("expected 10 bytes tracked, got %d", pw.bar.current)
	}
}

func TestProgressWriter_Finish(t *testing.T) {
	var buf bytes.Buffer
	pw := NewProgressWriter(&buf, 100, "writing")
	pw.bar.writer = io.Discard
	pw.Finish()
	if pw.bar.current != pw.bar.total {
		t.Errorf("expected current == total after Finish")
	}
}
