package alx

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

// The test binary doubles as alx, exercising real child processes and argv on
// every platform without a shell dependency.
func TestMain(m *testing.M) {
	if os.Getenv("ALX_CLIENT_TEST_HELPER") == "1" {
		os.Exit(runFakeALX())
	}
	os.Exit(m.Run())
}

func runFakeALX() int {
	args := os.Args[1:]
	log, err := os.OpenFile(os.Getenv("FAKE_LOG"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	err = json.NewEncoder(log).Encode(args)
	_ = log.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if len(args) >= 2 && args[0] == "--config" {
		args = args[2:]
	}
	switch os.Getenv("FAKE_SCENARIO") {
	case "version-output":
		if len(args) != 1 || args[0] != "--version" {
			return 2
		}
		fmt.Println(os.Getenv("FAKE_VERSION"))
		return 0
	case "json-commands":
		if len(args) == 0 {
			break
		}
		switch args[0] {
		case "--version":
			fmt.Println("alx 9.9.9")
		case "search":
			fmt.Println(`[{"cli_id":"c1","song_id":"s1","name":"晴天","singer":"周杰伦","source":"wy","interval":"04:29","album_name":"叶惠美"}]`)
		case "state":
			fmt.Println(`{"current_song":null,"daemon_active":false,"duration":0,"position":0,"queue_length":0,"repeat":"off","shuffle":false,"status":"stopped","volume":0}`)
		case "playlist":
			if len(args) < 2 || args[1] != "list" {
				break
			}
			fmt.Println(`[["Favorites","Default",2]]`)
		case "source":
			fmt.Println("No custom music sources registered yet.")
		case "lyric":
			fmt.Println(`{"cli_id":"c1","lyric":"[00:01.00]hi"}`)
		case "config":
			fmt.Println(`{}`)
		default:
			fmt.Fprintln(os.Stderr, "unexpected arguments:", args)
			return 2
		}
		return 0
	case "error-json":
		fmt.Println(`{"error":"no playback is running"}`)
		return 1
	case "empty-json":
		return 0
	case "cache-retry":
		if len(args) >= 2 && args[0] == "playlist" && args[1] == "add" {
			marker := filepath.Join(os.Getenv("FAKE_STATE"), "retried")
			if _, err := os.Stat(marker); err == nil {
				fmt.Println(`{"playlist":"P","songs":["c1"],"status":"added"}`)
			} else if err := os.WriteFile(marker, nil, 0o600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 2
			} else {
				fmt.Println(`{"error":"Song with CLI ID 'abc' not found in cache. Search first."}`)
			}
			return 0
		}
		if len(args) > 0 && args[0] == "search" {
			fmt.Println(`[{"cli_id":"c1","song_id":"s1","name":"晴天","singer":"周杰伦","source":"wy","interval":"04:29"}]`)
			return 0
		}
	case "quit":
		fmt.Println(`{"status":"exited"}`)
		return 0
	case "import-success":
		fmt.Println(`{"name":"我的夜行","count":42,"source":"tx","warnings":["跳过 1 首不可用歌曲"]}`)
		return 0
	case "import-error":
		fmt.Println(`{"error":"不支持的歌单链接"}`)
		return 1
	case "import-block":
		time.Sleep(30 * time.Second)
		return 0
	}
	fmt.Fprintln(os.Stderr, "unexpected arguments:", args)
	return 2
}

func newFakeClient(t *testing.T, scenario string) (*Client, string, string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.jsonl")
	env := append(os.Environ(), "ALX_CLIENT_TEST_HELPER=1", "FAKE_SCENARIO="+scenario, "FAKE_LOG="+logPath, "FAKE_STATE="+dir)
	return &Client{Bin: exe, Env: env}, logPath, dir
}

func assertCalls(t *testing.T, logPath string, want [][]string) {
	t.Helper()
	log, err := os.Open(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	var got [][]string
	scanner := bufio.NewScanner(log)
	for scanner.Scan() {
		var args []string
		if err := json.Unmarshal(scanner.Bytes(), &args); err != nil {
			t.Fatal(err)
		}
		got = append(got, args)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command argv = %#v, want %#v", got, want)
	}
}

func executableName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

func writeFakeExecutable(t *testing.T, dir, base string) string {
	t.Helper()
	path := filepath.Join(dir, executableName(base))
	mode := os.FileMode(0o700)
	if runtime.GOOS == "windows" {
		mode = 0o600 // Windows executability comes from the extension, not 0111.
	}
	if err := os.WriteFile(path, []byte("fixture"), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func changeDirectory(t *testing.T, path string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
}
