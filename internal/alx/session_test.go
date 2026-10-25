package alx

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestSessionCancellationStopsInFlightAndPreventsLaterCommands(t *testing.T) {
	c, logPath, _ := newFakeClient(t, "import-block")
	managed, err := NewClient(c.Bin, "")
	if err != nil {
		t.Fatal(err)
	}
	managed.Env = c.Env
	c = managed
	session, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.SessionContext = session
	result := make(chan error, 1)
	go func() { _, err := c.Play(context.Background(), "old-song"); result <- err }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if st, err := os.Stat(logPath); err == nil && st.Size() > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("backend subprocess never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	idleCtx, idleCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer idleCancel()
	if err := c.WaitIdle(idleCtx); err != nil {
		t.Fatalf("wait for ended backend commands: %v", err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("in-flight command error=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled backend command remained alive")
	}
	if _, err := c.State(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("ended session executed command: %v", err)
	}
	assertCalls(t, logPath, [][]string{{"play", "old-song", "--json", "--no-color"}})
}
