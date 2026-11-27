package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dundee/go-architect/internal/gitcmd"
	"google.golang.org/protobuf/proto"
)

type Store struct {
	dir string
}

func DefaultDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir: %w", err)
	}
	return filepath.Join(base, "go-architect", "v4"), nil
}

func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) DirForTest() string {
	return s.dir
}

func ProjectKey(modulePkg, subPath string, orgPackages []string) string {
	sortedOrg := make([]string, 0, len(orgPackages))
	for _, p := range orgPackages {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			sortedOrg = append(sortedOrg, trimmed)
		}
	}
	sort.Strings(sortedOrg)
	return fmt.Sprintf("%s::%s::%s", modulePkg, subPath, strings.Join(sortedOrg, ","))
}

func ResolveCleanCommit(ctx context.Context, projectPath string) (string, bool) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", false
	}

	shaOut, err := gitcmd.CommandContext(ctx, absPath, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", false
	}
	sha := strings.TrimSpace(string(shaOut))
	if sha == "" {
		return "", false
	}

	statusOut, err := gitcmd.CommandContext(ctx, absPath, "status", "--porcelain").Output()
	if err != nil {
		return "", false
	}
	if strings.TrimSpace(string(statusOut)) != "" {
		return "", false
	}

	return sha, true
}

func ProjectSubdir(ctx context.Context, projectPath string) string {
	absProj, err := filepath.Abs(projectPath)
	if err != nil {
		return "."
	}
	out, err := gitcmd.CommandContext(ctx, absProj, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "."
	}
	gitRoot := strings.TrimSpace(string(out))
	if evalRoot, e := filepath.EvalSymlinks(gitRoot); e == nil {
		gitRoot = evalRoot
	}
	if evalProj, e := filepath.EvalSymlinks(absProj); e == nil {
		absProj = evalProj
	}
	rel, err := filepath.Rel(gitRoot, absProj)
	if err != nil {
		return "."
	}
	return rel
}

func (s *Store) filePath(namespace, key string) string {
	sum := sha256.Sum256([]byte(key))
	digest := hex.EncodeToString(sum[:16])
	return filepath.Join(s.dir, fmt.Sprintf("%s_%s.pb", namespace, digest))
}

func (s *Store) GetProto(namespace, key string, dst proto.Message) bool {
	if s == nil || s.dir == "" || dst == nil {
		return false
	}
	data, err := os.ReadFile(s.filePath(namespace, key))
	if err != nil {
		return false
	}
	if err := proto.Unmarshal(data, dst); err != nil {
		return false
	}
	return true
}

func (s *Store) PutProto(namespace, key string, msg proto.Message) error {
	if s == nil || s.dir == "" || msg == nil {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	target := s.filePath(namespace, key)
	tmp, err := os.CreateTemp(s.dir, "cache-*.tmp")
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
