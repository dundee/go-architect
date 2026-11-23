package trends

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const snapshotSchemaVersion = "v1"

type Cache struct {
	dir string
}

func DefaultCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir: %w", err)
	}
	return filepath.Join(base, "go-architect", "snapshots"), nil
}

func NewCache(dir string) *Cache {
	return &Cache{dir: dir}
}

func (c *Cache) keyPath(projectKey, sha string) string {
	sum := sha256.Sum256([]byte(projectKey))
	shortProj := hex.EncodeToString(sum[:8])
	safeSHA := strings.TrimSpace(sha)
	return filepath.Join(c.dir, fmt.Sprintf("%s_%s_%s.json", snapshotSchemaVersion, shortProj, safeSHA))
}

func (c *Cache) Get(projectKey, sha string) (*CommitSnapshot, bool) {
	if c == nil || c.dir == "" {
		return nil, false
	}
	data, err := os.ReadFile(c.keyPath(projectKey, sha))
	if err != nil {
		return nil, false
	}
	var snap CommitSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, false
	}
	return &snap, true
}

func (c *Cache) Put(projectKey string, snap CommitSnapshot) error {
	if c == nil || c.dir == "" {
		return nil
	}
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}
	snap.Cached = false
	payload, err := json.Marshal(snap)
	if err != nil {
		return err
	}

	target := c.keyPath(projectKey, snap.CommitSHA)
	tmp, err := os.CreateTemp(c.dir, "snap-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(payload); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, target)
}
