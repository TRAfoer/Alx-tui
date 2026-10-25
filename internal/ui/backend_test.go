package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"alxtui/internal/alx"
)

// Run the real Go test executable as a CLI double so subprocess tests work on
// Windows and Unix without relying on an installed shell.
func TestMain(m *testing.M) {
	if os.Getenv("ALX_UI_TEST_PROFILE_HELPER") == "1" {
		runProfileHelper()
	}
	if os.Getenv("ALX_UI_TEST_HELPER") == "1" {
		if path := os.Getenv("ALX_UI_TEST_LOG"); path != "" {
			file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			_ = json.NewEncoder(file).Encode(os.Args[1:])
			_ = file.Close()
		}
		fmt.Fprint(os.Stdout, os.Getenv("ALX_UI_TEST_STDOUT"))
		code, _ := strconv.Atoi(os.Getenv("ALX_UI_TEST_EXIT"))
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func runProfileHelper() {
	args := os.Args[1:]
	name := filepath.Base(os.Args[0])
	if path := os.Getenv("ALX_UI_TEST_PROFILE_LOG"); path != "" {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(2)
		}
		_ = json.NewEncoder(file).Encode(profileCall{Bin: name, Args: args})
		_ = file.Close()
	}
	if len(args) == 1 && args[0] == "--version" {
		fmt.Println("Lx-Cli 0.1.1-go")
		os.Exit(0)
	}
	config := ""
	if len(args) >= 2 && args[0] == "--config" {
		config, args = args[1], args[2:]
	}
	if len(args) < 1 {
		os.Exit(3)
	}
	switch args[0] {
	case "config":
		if len(args) > 1 && args[1] == "path" {
			fmt.Println(config)
			os.Exit(0)
		}
		if os.Getenv("ALX_UI_TEST_INVALID_CONFIG") == name {
			fmt.Fprintln(os.Stderr, "invalid config")
			os.Exit(4)
		}
		if os.Getenv("ALX_UI_TEST_BLOCK_CONFIG") == name {
			_ = os.WriteFile(os.Getenv("ALX_UI_TEST_STARTED"), []byte("started"), 0o600)
			time.Sleep(time.Minute)
		}
		if config != "" {
			_ = os.MkdirAll(filepath.Dir(config), 0o700)
			_ = os.WriteFile(config, []byte("[source]\n"), 0o600)
		}
		fmt.Println(`{"source":{"default_source":"wy","default_quality":"320k"}}`)
	case "source":
		fmt.Printf(`[{"id":"%s-source","name":"Test","enabled":true}]`, name)
	case "playlist":
		fmt.Printf(`[{"name":"%s-library","count":0}]`, name)
	case "state":
		fmt.Println(`{"status":"stopped"}`)
	case "queue":
		fmt.Println(`{"songs":[],"current_index":0}`)
	default:
		fmt.Println(`{}`)
	}
	os.Exit(0)
}

type profileCall struct {
	Bin  string
	Args []string
}

func testBackend(t *testing.T, stdout string, exitCode int) (*alx.Client, string) {
	t.Helper()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "calls.jsonl")
	return &alx.Client{Bin: bin, Env: append(os.Environ(),
		"ALX_UI_TEST_HELPER=1", "ALX_UI_TEST_LOG="+logPath,
		"ALX_UI_TEST_STDOUT="+stdout, "ALX_UI_TEST_EXIT="+strconv.Itoa(exitCode),
	)}, logPath
}

func testBackendArgs(t *testing.T, logPath string) []string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	if err := json.Unmarshal(data, &args); err != nil {
		t.Fatal(err)
	}
	return args
}
