package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	shared "github.com/asolopovas/wt/internal"
)

type Status string

const (
	StatusNotInstalled Status = "not_installed"
	StatusDownloading  Status = "downloading"
	StatusInstalled    Status = "installed"
	StatusCorrupt      Status = "corrupt"
)

type installedFile struct {
	RelPath   string `json:"relPath"`
	SizeBytes int64  `json:"sizeBytes"`
	SHA256    string `json:"sha256"`
}

type installedManifest struct {
	SchemaVersion int             `json:"schemaVersion"`
	ID            string          `json:"id"`
	Family        Family          `json:"family"`
	Engine        string          `json:"engine,omitempty"`
	DisplayName   string          `json:"displayName"`
	InstalledAt   string          `json:"installedAt"`
	Files         []installedFile `json:"files"`
}

type Progress struct {
	ID         string
	Downloaded int64
	Total      int64
	Err        error
	Done       bool
}

type Manager struct {
	mu      sync.Mutex
	active  map[Family]string
	jobs    map[string]*job
	maxPar  int
	running int
}

type job struct {
	id     string
	cancel context.CancelFunc
}

const defaultParallel = 2

var (
	sharedManagerOnce sync.Once
	sharedManager     *Manager
)

func NewManager() *Manager {
	sharedManagerOnce.Do(func() {
		sharedManager = &Manager{
			active: map[Family]string{},
			jobs:   map[string]*job{},
			maxPar: defaultParallel,
		}
		sharedManager.loadActive()
	})
	return sharedManager
}

func (m *Manager) Status(id string) Status {
	e, ok := ByID(id)
	if !ok {
		return StatusNotInstalled
	}
	m.mu.Lock()
	_, downloading := m.jobs[id]
	m.mu.Unlock()
	if downloading {
		return StatusDownloading
	}
	for _, p := range PathsFor(e) {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			return StatusNotInstalled
		}
	}
	return StatusInstalled
}

func (m *Manager) Active(f Family) string {
	m.mu.Lock()
	if id, ok := m.active[f]; ok {

		m.mu.Unlock()
		return id
	}
	m.mu.Unlock()

	entries := ByFamily(f)

	for _, e := range entries {
		if !e.DefaultActive {
			continue
		}
		installed := true
		for _, p := range PathsFor(e) {
			if !fileExists(p) {
				installed = false
				break
			}
		}
		if installed {
			return e.ID
		}
	}

	for _, e := range entries {
		installed := true
		for _, p := range PathsFor(e) {
			if !fileExists(p) {
				installed = false
				break
			}
		}
		if installed {
			return e.ID
		}
	}

	for _, e := range entries {
		if e.DefaultActive {
			return e.ID
		}
	}
	return ""
}

func (m *Manager) SetActive(id string) error {
	e, ok := ByID(id)
	if !ok {
		return fmt.Errorf("unknown model: %s", id)
	}
	for _, p := range PathsFor(e) {
		if !fileExists(p) {
			return fmt.Errorf("model not installed: %s", id)
		}
	}
	m.mu.Lock()
	m.active[Family(e.Family)] = id
	m.mu.Unlock()
	return m.saveActive()
}

func (m *Manager) ClearActive(f Family) error {
	m.mu.Lock()
	if id, ok := m.active[f]; ok && id == "" {
		m.mu.Unlock()
		return nil
	}
	m.active[f] = ""
	m.mu.Unlock()
	return m.saveActive()
}

func (m *Manager) Get(ctx context.Context, id string, prog func(Progress)) error {
	e, ok := ByID(id)
	if !ok {
		return fmt.Errorf("unknown model: %s", id)
	}

	specs := e.Files
	paths := PathsFor(e)

	var totalAll int64
	for _, s := range specs {
		totalAll += s.SizeBytes
	}
	if totalAll <= 0 {
		totalAll = e.SizeBytes
	}

	allPresent := true
	for _, p := range paths {
		if !fileExists(p) {
			allPresent = false
			break
		}
	}
	if allPresent {
		log.Printf("[models] %s: already installed (%d files, %.1f MB)", id, len(paths), float64(totalAll)/(1024*1024))
		if prog != nil {
			prog(Progress{ID: id, Downloaded: totalAll, Total: totalAll, Done: true})
		}
		return nil
	}

	if err := m.acquireSlot(ctx, id); err != nil {
		return err
	}
	defer m.releaseSlot(id)

	log.Printf("[models] %s: downloading %d file(s), %.1f MB total", id, len(specs), float64(totalAll)/(1024*1024))
	display := e.DisplayName
	if display == "" {
		display = id
	}
	shared.LogInfo(fmt.Sprintf("Downloading model: %s (%s, %d file(s), %.0f MB)", display, id, len(specs), float64(totalAll)/(1024*1024)))
	start := time.Now()
	var completed int64
	for i, s := range specs {
		dst := paths[i]
		if fileExists(dst) {
			log.Printf("[models] %s: skip file %d/%d %s (already present)", id, i+1, len(specs), filepath.Base(dst))
			completed += s.SizeBytes
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			log.Printf("[models] %s: FAILED mkdir %s: %v", id, filepath.Dir(dst), err)
			return err
		}
		log.Printf("[models] %s: file %d/%d %s (%.1f MB) <- %s", id, i+1, len(specs), filepath.Base(dst), float64(s.SizeBytes)/(1024*1024), s.URL)
		fileStart := time.Now()
		base := completed
		fileTotal := s.SizeBytes
		cb := shared.DownloadProgress(func(downloaded, total int64) {
			if prog == nil {
				return
			}
			ft := fileTotal
			if total > 0 {
				ft = total
			}
			prog(Progress{ID: id, Downloaded: base + downloaded, Total: completed + ft + (totalAll - completed - fileTotal)})
		})
		if err := shared.DownloadFile(dst, s.URL, cb); err != nil {
			log.Printf("[models] %s: FAILED file %d/%d %s: %v", id, i+1, len(specs), filepath.Base(dst), err)
			shared.LogError(fmt.Sprintf("  %s: failed downloading %s: %v", display, filepath.Base(dst), err))
			return fmt.Errorf("downloading %s: %w", id, err)
		}
		if s.SHA256 != "" {
			if err := verifySHA256(dst, s.SHA256); err != nil {
				log.Printf("[models] %s: FAILED sha256 mismatch for %s: %v", id, filepath.Base(dst), err)
				_ = os.Remove(dst)
				return fmt.Errorf("verifying %s: %w", id, err)
			}
		}
		log.Printf("[models] %s: file %d/%d %s done (%.1fs)", id, i+1, len(specs), filepath.Base(dst), time.Since(fileStart).Seconds())
		shared.LogInfo(fmt.Sprintf("  %s: %s (%.0f MB) %.0fs", display, filepath.Base(dst), float64(s.SizeBytes)/(1024*1024), time.Since(fileStart).Seconds()))
		completed += s.SizeBytes
	}

	if err := writeInstalledManifest(e); err != nil {
		log.Printf("[models] %s: FAILED model.json write: %v", id, err)
		return fmt.Errorf("writing model.json for %s: %w", id, err)
	}
	log.Printf("[models] %s: installed in %.1fs", id, time.Since(start).Seconds())
	shared.LogInfo(fmt.Sprintf("Installed model: %s in %.0fs", display, time.Since(start).Seconds()))

	if prog != nil {
		prog(Progress{ID: id, Downloaded: totalAll, Total: totalAll, Done: true})
	}

	m.mu.Lock()
	cur := m.active[Family(e.Family)]
	m.mu.Unlock()
	if cur == "" {
		_ = m.SetActive(id)
	}
	return nil
}

func installedManifestPath(e Entry) string {
	dir := filepath.Join(shared.ModelsDir(), e.ID)
	return filepath.Join(dir, "model.json")
}

func writeInstalledManifest(e Entry) error {
	instPath := installedManifestPath(e)
	if err := os.MkdirAll(filepath.Dir(instPath), 0o755); err != nil {
		return err
	}
	specs := e.Files
	paths := PathsFor(e)
	files := make([]installedFile, 0, len(specs))
	for i, s := range specs {
		hash := s.SHA256
		if hash == "" {
			computed, err := computeSHA256(paths[i])
			if err != nil {
				return fmt.Errorf("hashing %s: %w", paths[i], err)
			}
			hash = computed
		}
		size := s.SizeBytes
		if st, err := os.Stat(paths[i]); err == nil {
			size = st.Size()
		}
		files = append(files, installedFile{RelPath: s.RelPath, SizeBytes: size, SHA256: hash})
	}
	im := installedManifest{
		SchemaVersion: 1,
		ID:            e.ID,
		Family:        Family(e.Family),
		Engine:        e.Engine,
		DisplayName:   e.DisplayName,
		InstalledAt:   nowRFC3339(),
		Files:         files,
	}
	data, err := json.MarshalIndent(im, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(instPath, data, 0o644)
}

func readInstalledManifest(e Entry) (*installedManifest, error) {
	data, err := os.ReadFile(installedManifestPath(e))
	if err != nil {
		return nil, err
	}
	var im installedManifest
	if err := json.Unmarshal(data, &im); err != nil {
		return nil, err
	}
	return &im, nil
}

func (m *Manager) Verify(id string) error {
	e, ok := ByID(id)
	if !ok {
		return fmt.Errorf("unknown model: %s", id)
	}
	specs := e.Files
	paths := PathsFor(e)
	inst, _ := readInstalledManifest(e)
	for i, s := range specs {
		if !fileExists(paths[i]) {
			return fmt.Errorf("missing file %s", paths[i])
		}
		want := s.SHA256
		if want == "" && inst != nil {
			for _, f := range inst.Files {
				if f.RelPath == s.RelPath {
					want = f.SHA256
					break
				}
			}
		}
		if want == "" {
			continue
		}
		if err := verifySHA256(paths[i], want); err != nil {
			return fmt.Errorf("%s: %w", s.RelPath, err)
		}
	}
	return nil
}

func computeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func nowRFC3339() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func (m *Manager) Cancel(id string) {
	m.mu.Lock()
	j := m.jobs[id]
	m.mu.Unlock()
	if j != nil {
		j.cancel()
	}
}

func (m *Manager) Delete(id string) error {
	e, ok := ByID(id)
	if !ok {
		return fmt.Errorf("unknown model: %s", id)
	}

	shared := map[string]bool{}
	for _, other := range Catalog() {
		if other.ID == e.ID {
			continue
		}
		for _, p := range PathsFor(other) {
			shared[p] = true
		}
	}
	any := false
	for _, p := range PathsFor(e) {
		if !fileExists(p) {
			continue
		}
		any = true
		if shared[p] {
			continue
		}
		if err := os.Remove(p); err != nil {
			return err
		}
	}
	if !any {
		return nil
	}
	m.mu.Lock()
	if m.active[Family(e.Family)] == id {
		delete(m.active, Family(e.Family))
	}
	m.mu.Unlock()
	return m.saveActive()
}

type Orphan struct {
	Path      string
	SizeBytes int64
}

func (m *Manager) Orphans() []Orphan {
	known := map[string]bool{}
	for _, e := range Catalog() {
		for _, p := range PathsFor(e) {
			known[filepath.Clean(p)] = true
		}
		known[filepath.Clean(installedManifestPath(e))] = true
		known[filepath.Clean(filepath.Dir(installedManifestPath(e)))] = true
		for _, p := range PathsFor(e) {
			known[filepath.Clean(filepath.Dir(p))] = true
		}
	}
	roots := []string{shared.ModelsDir(), filepath.Join(externalRoot(), "llm")}
	seen := map[string]bool{}
	var out []Orphan
	for _, root := range roots {
		if seen[root] {
			continue
		}
		seen[root] = true
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			abs := filepath.Join(root, ent.Name())
			if known[filepath.Clean(abs)] {
				continue
			}
			size := dirOrFileSize(abs)
			out = append(out, Orphan{Path: abs, SizeBytes: size})
		}
	}
	return out
}

func (m *Manager) RemoveOrphan(path string) error {
	cleaned := filepath.Clean(path)
	protected := map[string]bool{}
	for _, root := range []string{shared.ModelsDir(), filepath.Join(externalRoot(), "llm")} {
		protected[filepath.Clean(root)] = true
	}
	if protected[cleaned] {
		return fmt.Errorf("refusing to delete root: %s", cleaned)
	}
	inside := false
	for root := range protected {
		rel, err := filepath.Rel(root, cleaned)
		if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
			inside = true
			break
		}
	}
	if !inside {
		return fmt.Errorf("refusing to delete outside models dir: %s", cleaned)
	}
	return os.RemoveAll(cleaned)
}

func dirOrFileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	if !st.IsDir() {
		return st.Size()
	}
	var total int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() {
			return nil //nolint:nilerr // best-effort sizing
		}
		total += info.Size()
		return nil
	})
	return total
}

func (m *Manager) DiskUsage() int64 {
	var total int64
	for _, e := range Catalog() {
		for _, p := range PathsFor(e) {
			if st, err := os.Stat(p); err == nil {
				total += st.Size()
			}
		}
	}
	return total
}

func (m *Manager) acquireSlot(ctx context.Context, id string) error {
	for {
		m.mu.Lock()
		if m.running < m.maxPar {
			m.running++
			jctx, cancel := context.WithCancel(ctx)
			m.jobs[id] = &job{id: id, cancel: cancel}
			_ = jctx
			m.mu.Unlock()
			return nil
		}
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func (m *Manager) releaseSlot(id string) {
	m.mu.Lock()
	delete(m.jobs, id)
	if m.running > 0 {
		m.running--
	}
	m.mu.Unlock()
}

func activeFile() string {
	return filepath.Join(shared.Dir(), "active-models.json")
}

func (m *Manager) loadActive() {
	data, err := os.ReadFile(activeFile())
	if err != nil {
		return
	}
	raw := map[string]string{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	for k, v := range raw {
		if _, ok := ByID(v); !ok {
			v = DefaultID(Family(k))
		}
		m.active[Family(k)] = v
	}
}

func (m *Manager) saveActive() error {
	m.mu.Lock()
	raw := map[string]string{}
	for k, v := range m.active {
		raw[string(k)] = v
	}
	m.mu.Unlock()
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(activeFile()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(activeFile(), data, 0o644)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func verifySHA256(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("hash mismatch: got %s want %s", got, want)
	}
	return nil
}

var ErrAlreadyInstalled = errors.New("already installed")

func (m *Manager) EnsureDefaults(ctx context.Context, prog func(Progress)) error {
	log.Printf("[models] EnsureDefaults: checking family defaults")
	for _, f := range []Family{FamilyASR, FamilyDiarizer} {
		id := DefaultID(f)
		if id == "" {
			log.Printf("[models] EnsureDefaults: family %s has no default; skip", f)
			continue
		}
		switch m.Status(id) {
		case StatusInstalled:
			log.Printf("[models] EnsureDefaults: %s default %q already installed", f, id)
			continue
		case StatusDownloading:
			log.Printf("[models] EnsureDefaults: %s default %q already downloading; skip", f, id)
			continue
		}
		log.Printf("[models] EnsureDefaults: fetching %s default %q", f, id)
		if err := m.Get(ctx, id, prog); err != nil {
			log.Printf("[models] EnsureDefaults: FAILED %s default %q: %v", f, id, err)
			return fmt.Errorf("ensuring %s default %q: %w", f, id, err)
		}
	}
	log.Printf("[models] EnsureDefaults: complete")
	return nil
}
