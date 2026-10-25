package profile

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type legacyPaths struct{ config, data, db string }

func (m *Manager) migrate(ctx context.Context, stage, target string, env []string) (bool, error) {
	old, err := resolveLegacy(env)
	if err != nil {
		return false, err
	}
	migrated := false
	if !within(m.Root, old.config) {
		copied, err := copyOptional(old.config, filepath.Join(stage, "config.toml"))
		if err != nil {
			return false, err
		}
		migrated = copied
	}
	if !within(m.Root, old.data) {
		sources := filepath.Join(old.data, "sources")
		if info, err := os.Lstat(sources); err == nil {
			if !info.IsDir() {
				return false, fmt.Errorf("旧音源目录不是常规目录: %s", sources)
			}
			if err := copySources(ctx, sources, filepath.Join(stage, "data", "sources"), m.Root); err != nil {
				return false, err
			}
			migrated = true
		} else if !os.IsNotExist(err) {
			return false, err
		}
		dbPath := filepath.Join(old.data, old.db)
		if info, err := os.Lstat(dbPath); err == nil {
			if !info.Mode().IsRegular() {
				return false, fmt.Errorf("旧数据库不是常规文件: %s", dbPath)
			}
			destination := filepath.Join(stage, "data", old.db)
			if err := snapshotSQLite(ctx, dbPath, destination); err != nil {
				return false, err
			}
			if err := rewriteSourcePaths(ctx, destination, sources, filepath.Join(target, "data", "sources")); err != nil {
				return false, err
			}
			migrated = true
		} else if !os.IsNotExist(err) {
			return false, err
		}
	}
	return migrated, nil
}

// The standalone Go CLI still uses alx-go and honors legacy ALX_HOME.
func resolveLegacy(env []string) (legacyPaths, error) {
	configBase, err := os.UserConfigDir()
	if err != nil {
		return legacyPaths{}, err
	}
	base := filepath.Join(configBase, "alx-go")
	if home := envValue(env, "ALX_HOME"); home != "" {
		base = home
	}
	config, data := filepath.Join(base, "config.toml"), filepath.Join(base, "data")
	if value := envValue(env, "ALX_CONFIG"); value != "" {
		config = value
	}
	if value := envValue(env, "ALX_DATA"); value != "" {
		data = value
	}
	config, err = absolutePath(config)
	if err != nil {
		return legacyPaths{}, err
	}
	data, err = absolutePath(data)
	if err != nil {
		return legacyPaths{}, err
	}
	return legacyPaths{config: config, data: data, db: "alx-go.db"}, nil
}

// Only known persistent files are copied. Runtime state and cache trees are
// intentionally excluded, and symlinks are rejected rather than followed out
// of the managed source tree.
func copySources(ctx context.Context, from, to, portableRoot string) error {
	return filepath.WalkDir(from, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == from {
			return nil
		}
		// An explicitly configured portable root may itself be inside the old
		// sources tree. Never walk our destination while producing its copy.
		if within(portableRoot, path) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&(os.ModeSocket|os.ModeNamedPipe) != 0 {
			return nil
		}
		if runtimeEntry(entry.Name()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0700)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("迁移不跟随音源符号链接: %s", path)
		}
		_, err = copyOptional(path, destination)
		return err
	})
}

func runtimeEntry(name string) bool {
	name = strings.ToLower(name)
	switch name {
	case "cache", ".cache", "caches", "daemon.json", "daemon", "socket", "pid", "lock":
		return true
	}
	for _, suffix := range []string{".lock", ".pid", ".sock", ".socket", "-wal", "-shm"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func copyOptional(from, to string) (bool, error) {
	info, err := os.Lstat(from)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("迁移文件不是常规文件: %s", from)
	}
	source, err := os.Open(from)
	if err != nil {
		return false, err
	}
	defer source.Close()
	destination, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return false, err
	}
	_, copyErr := io.Copy(destination, source)
	if copyErr == nil {
		copyErr = destination.Sync()
	}
	closeErr := destination.Close()
	if copyErr != nil {
		return false, copyErr
	}
	if closeErr != nil {
		return false, closeErr
	}
	return true, nil
}

func sqliteURL(path string, readonly bool) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := url.URL{Scheme: "file", Path: path}
	query := url.Values{"_pragma": []string{"busy_timeout(5000)"}}
	if readonly {
		query.Set("mode", "ro")
	}
	u.RawQuery = query.Encode()
	return u.String()
}

// VACUUM INTO reads a consistent SQLite snapshot, including committed WAL
// pages. Reading the raw main file would silently lose those transactions.
func snapshotSQLite(ctx context.Context, source, destination string) error {
	db, err := sql.Open("sqlite", sqliteURL(source, true))
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, "VACUUM INTO ?", destination)
	if err != nil {
		return fmt.Errorf("SQLite快照 %s: %w", source, err)
	}
	return nil
}

func rewriteSourcePaths(ctx context.Context, path, oldSources, newSources string) error {
	db, err := sql.Open("sqlite", sqliteURL(path, false))
	if err != nil {
		return err
	}
	defer db.Close()
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='sources'").Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	rows, err := db.QueryContext(ctx, "SELECT rowid, payload FROM sources")
	if err != nil {
		return err
	}
	type update struct {
		row   int64
		value string
	}
	var updates []update
	for rows.Next() {
		var row int64
		var value string
		if err := rows.Scan(&row, &value); err != nil {
			_ = rows.Close()
			return err
		}
		next := value
		var payload map[string]json.RawMessage
		if err := json.Unmarshal([]byte(value), &payload); err != nil {
			_ = rows.Close()
			return fmt.Errorf("旧音源JSON无效: %w", err)
		}
		var script string
		if raw, ok := payload["script_path"]; ok {
			if err := json.Unmarshal(raw, &script); err != nil {
				_ = rows.Close()
				return err
			}
			if rewritten := relocated(script, oldSources, newSources); rewritten != script {
				payload["script_path"], _ = json.Marshal(rewritten)
				encoded, err := json.Marshal(payload)
				if err != nil {
					_ = rows.Close()
					return err
				}
				next = string(encoded)
			}
		}
		if next != value {
			updates = append(updates, update{row: row, value: next})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, update := range updates {
		if _, err := tx.ExecContext(ctx, "UPDATE sources SET payload=? WHERE rowid=?", update.value, update.row); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func within(root, path string) bool {
	if root == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func relocated(path, oldRoot, newRoot string) string {
	if !filepath.IsAbs(path) || !within(oldRoot, path) {
		return path
	}
	rel, err := filepath.Rel(oldRoot, path)
	if err != nil || rel == "." {
		return path
	}
	return filepath.Join(newRoot, rel)
}
