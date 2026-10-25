package ui

import (
	"reflect"
	"testing"
)

func TestAppShutdownCallsAlxQuit(t *testing.T) {
	client, logPath := testBackend(t, `{"status":"exited"}`, 0)
	app := &App{client: client}
	if err := app.Shutdown(); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}
	if got, want := testBackendArgs(t, logPath), []string{"quit", "--json", "--no-color"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("alx args = %q, want %q", got, want)
	}
}

func TestAppShutdownNilSafe(t *testing.T) {
	var app *App
	if err := app.Shutdown(); err != nil {
		t.Fatalf("nil app Shutdown: %v", err)
	}
	empty := &App{}
	if err := empty.Shutdown(); err != nil {
		t.Fatalf("empty app Shutdown: %v", err)
	}
}
