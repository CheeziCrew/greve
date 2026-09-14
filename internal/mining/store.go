package mining

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// Store appends kept comments to an NDJSON file and tracks per-repo resume
// progress in a sibling JSON file. It dedups by comment id across runs by
// loading existing ids on open.
type Store struct {
	path     string
	progress string
	seen     map[string]bool
	f        *os.File
	w        *bufio.Writer
}

// Progress maps a repo key ("owner/name") to its last GraphQL cursor, or the
// sentinel "done" when fully mined.
type Progress map[string]string

func dataDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	d := filepath.Join(dir, "greve")
	if err := os.MkdirAll(d, 0o755); err != nil {
		return "", err
	}
	return d, nil
}

// OpenStore opens (creating if needed) the NDJSON store at path, or the default
// ~/<cache>/greve/reviews.ndjson when path is "".
func OpenStore(path string) (*Store, error) {
	if path == "" {
		d, err := dataDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(d, "reviews.ndjson")
	}
	s := &Store{path: path, progress: path + ".progress.json", seen: map[string]bool{}}

	if f, err := os.Open(path); err == nil {
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
		for sc.Scan() {
			var c Comment
			if json.Unmarshal(sc.Bytes(), &c) == nil && c.ID != "" {
				s.seen[c.ID] = true
			}
		}
		_ = f.Close()
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	s.f = f
	s.w = bufio.NewWriter(f)
	return s, nil
}

func (s *Store) Path() string       { return s.path }
func (s *Store) Has(id string) bool { return s.seen[id] }
func (s *Store) Count() int         { return len(s.seen) }

// Append writes a comment unless its id is already stored.
func (s *Store) Append(c Comment) error {
	if s.seen[c.ID] {
		return nil
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if _, err := s.w.Write(append(b, '\n')); err != nil {
		return err
	}
	s.seen[c.ID] = true
	return nil
}

func (s *Store) Flush() error { return s.w.Flush() }

func (s *Store) Close() error {
	if err := s.w.Flush(); err != nil {
		return err
	}
	return s.f.Close()
}

func (s *Store) LoadProgress() Progress {
	p := Progress{}
	if data, err := os.ReadFile(s.progress); err == nil {
		_ = json.Unmarshal(data, &p)
	}
	return p
}

func (s *Store) SaveProgress(p Progress) {
	if data, err := json.MarshalIndent(p, "", "  "); err == nil {
		_ = os.WriteFile(s.progress, data, 0o644)
	}
}
