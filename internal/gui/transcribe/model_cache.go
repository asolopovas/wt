package transcribe

import (
	"fmt"
	"os"
	"strconv"
	"time"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

const (
	defaultUnloadSec = 300
	watcherTickSec   = 30
)

func unloadTimeout() time.Duration {
	if v := os.Getenv("WT_MODEL_UNLOAD_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			if n <= 0 {
				return 0
			}
			return time.Duration(n) * time.Second
		}
	}
	return defaultUnloadSec * time.Second
}

func (p *Panel) acquireModel(modelSize string) (whisper.Model, error) {
	p.modelMu.Lock()
	defer p.modelMu.Unlock()
	p.touchModelActivity()

	if p.cachedModel != nil && p.cachedModelSize == modelSize {
		p.debugLog(fmt.Sprintf("model cache hit (%s)", modelSize))
		return p.cachedModel, nil
	}

	if p.cachedModel != nil {
		p.debugLog(fmt.Sprintf("model size changed (%s -> %s); unloading old", p.cachedModelSize, modelSize))
		_ = p.cachedModel.Close()
		p.cachedModel = nil
		p.cachedModelSize = ""
	}

	m, err := p.loadModel(modelSize)
	if err != nil {
		return nil, err
	}
	p.cachedModel = m
	p.cachedModelSize = modelSize
	p.ensureUnloadWatcher()
	return m, nil
}

func (p *Panel) releaseModel() {
	p.touchModelActivity()
}

func (p *Panel) touchModelActivity() {
	p.lastModelActivity.Store(time.Now().UnixNano())
}

func (p *Panel) unloadCachedModel() {
	p.modelMu.Lock()
	defer p.modelMu.Unlock()
	if p.cachedModel == nil {
		return
	}
	p.debugLog(fmt.Sprintf("idle unload: dropping %s", p.cachedModelSize))
	_ = p.cachedModel.Close()
	p.cachedModel = nil
	p.cachedModelSize = ""
}

func (p *Panel) modelLoadedSize() string {
	p.modelMu.Lock()
	defer p.modelMu.Unlock()
	return p.cachedModelSize
}

func (p *Panel) ensureUnloadWatcher() {
	if !p.unloadWatcherStarted.CompareAndSwap(false, true) {
		return
	}
	timeout := unloadTimeout()
	if timeout <= 0 {
		return
	}
	stop := make(chan struct{})
	p.unloadWatcherStop = stop

	go func() {
		ticker := time.NewTicker(watcherTickSec * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				p.mu.Lock()
				running := p.running
				p.mu.Unlock()
				if running {
					p.touchModelActivity()
					continue
				}
				if p.modelLoadedSize() == "" {
					continue
				}
				last := p.lastModelActivity.Load()
				if last == 0 {
					p.touchModelActivity()
					continue
				}
				idle := time.Since(time.Unix(0, last))
				if idle >= unloadTimeout() {
					p.unloadCachedModel()
				}
			}
		}
	}()
}

func (p *Panel) StopUnloadWatcher() {
	if p.unloadWatcherStop != nil {
		close(p.unloadWatcherStop)
		p.unloadWatcherStop = nil
	}
	p.unloadCachedModel()
}
