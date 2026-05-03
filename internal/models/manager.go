package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	shared "github.com/asolopovas/wt/internal"
)

type Status string

const (
	StatusNotInstalled Status = "not_installed"
	StatusDownloading  Status = "downloading"
	StatusInstalled    Status = "installed"
)

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

func NewManager() *Manager {
	m := &Manager{
		active: map[Family]string{},
		jobs:   map[string]*job{},
		maxPar: defaultParallel,
	}
	m.loadActive()
	return m
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
		if !fileExists(p) {
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
	m.active[e.Family] = id
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

	specs := e.FileSpecs()
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
		if prog != nil {
			prog(Progress{ID: id, Downloaded: totalAll, Total: totalAll, Done: true})
		}
		return nil
	}

	if err := m.acquireSlot(ctx, id); err != nil {
		return err
	}
	defer m.releaseSlot(id)

	var completed int64
	for i, s := range specs {
		dst := paths[i]
		if fileExists(dst) {
			completed += s.SizeBytes
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
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
			return fmt.Errorf("downloading %s: %w", id, err)
		}
		if s.SHA256 != "" {
			if err := verifySHA256(dst, s.SHA256); err != nil {
				_ = os.Remove(dst)
				return fmt.Errorf("verifying %s: %w", id, err)
			}
		}
		completed += s.SizeBytes
	}

	if prog != nil {
		prog(Progress{ID: id, Downloaded: totalAll, Total: totalAll, Done: true})
	}

	m.mu.Lock()
	cur := m.active[e.Family]
	m.mu.Unlock()
	if cur == "" {

		_ = m.SetActive(id)
	}
	return nil
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
	if m.active[e.Family] == id {
		delete(m.active, e.Family)
	}
	m.mu.Unlock()
	return m.saveActive()
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
		if mapped, ok := legacyDiarizerIDs[v]; ok {
			v = mapped
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

func SortedByFamily(entries []Entry) []Entry {
	out := append([]Entry{}, entries...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Family != out[j].Family {
			return out[i].Family < out[j].Family
		}
		return out[i].SizeBytes < out[j].SizeBytes
	})
	return out
}

var ErrAlreadyInstalled = errors.New("already installed")
