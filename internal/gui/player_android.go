//go:build android

package gui

/*
#cgo LDFLAGS: -llog

#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include <android/log.h>

#define WP_TAG "wtplay"
#define WP_LOGI(...) __android_log_print(ANDROID_LOG_INFO, WP_TAG, __VA_ARGS__)
#define WP_LOGE(...) __android_log_print(ANDROID_LOG_ERROR, WP_TAG, __VA_ARGS__)

static jobject g_player = NULL;

// wt_play_start: creates MediaPlayer, setDataSource(path), prepare(), start().
// Returns 1 on success.
static int wt_play_start(uintptr_t envPtr, const char* path) {
	JNIEnv* env = (JNIEnv*)envPtr;
	if (!env) return 0;

	if (g_player) {
		jclass cMP0 = (*env)->GetObjectClass(env, g_player);
		jmethodID mStop0  = (*env)->GetMethodID(env, cMP0, "stop", "()V");
		jmethodID mRel0   = (*env)->GetMethodID(env, cMP0, "release", "()V");
		if (mStop0) (*env)->CallVoidMethod(env, g_player, mStop0);
		if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
		if (mRel0)  (*env)->CallVoidMethod(env, g_player, mRel0);
		if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
		(*env)->DeleteGlobalRef(env, g_player);
		(*env)->DeleteLocalRef(env, cMP0);
		g_player = NULL;
	}

	jclass cMP = (*env)->FindClass(env, "android/media/MediaPlayer");
	if (!cMP) { (*env)->ExceptionClear(env); WP_LOGE("FindClass MediaPlayer failed"); return 0; }
	jmethodID mInit = (*env)->GetMethodID(env, cMP, "<init>", "()V");
	jobject mp = (*env)->NewObject(env, cMP, mInit);
	if (!mp) { (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, cMP); return 0; }

	jmethodID mSetSrc = (*env)->GetMethodID(env, cMP, "setDataSource", "(Ljava/lang/String;)V");
	jstring jPath = (*env)->NewStringUTF(env, path);
	(*env)->CallVoidMethod(env, mp, mSetSrc, jPath);
	(*env)->DeleteLocalRef(env, jPath);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); WP_LOGE("setDataSource failed"); goto fail; }

	jmethodID mPrep = (*env)->GetMethodID(env, cMP, "prepare", "()V");
	(*env)->CallVoidMethod(env, mp, mPrep);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); WP_LOGE("prepare failed"); goto fail; }

	jmethodID mStart = (*env)->GetMethodID(env, cMP, "start", "()V");
	(*env)->CallVoidMethod(env, mp, mStart);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); WP_LOGE("start failed"); goto fail; }

	g_player = (*env)->NewGlobalRef(env, mp);
	(*env)->DeleteLocalRef(env, mp);
	(*env)->DeleteLocalRef(env, cMP);
	WP_LOGI("playing: %s", path);
	return 1;

fail:
	{
		jmethodID mRel = (*env)->GetMethodID(env, cMP, "release", "()V");
		if (mRel) (*env)->CallVoidMethod(env, mp, mRel);
		if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
		(*env)->DeleteLocalRef(env, mp);
		(*env)->DeleteLocalRef(env, cMP);
	}
	return 0;
}

// wt_play_stop releases the active MediaPlayer.
static void wt_play_stop(uintptr_t envPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	if (!env || !g_player) return;
	jclass cMP = (*env)->GetObjectClass(env, g_player);
	jmethodID mStop = (*env)->GetMethodID(env, cMP, "stop", "()V");
	jmethodID mRel  = (*env)->GetMethodID(env, cMP, "release", "()V");
	if (mStop) (*env)->CallVoidMethod(env, g_player, mStop);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
	if (mRel)  (*env)->CallVoidMethod(env, g_player, mRel);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
	(*env)->DeleteGlobalRef(env, g_player);
	g_player = NULL;
	(*env)->DeleteLocalRef(env, cMP);
}

// wt_play_is_playing returns 1 if MediaPlayer.isPlaying() is true.
static int wt_play_is_playing(uintptr_t envPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	if (!env || !g_player) return 0;
	jclass cMP = (*env)->GetObjectClass(env, g_player);
	jmethodID mIs = (*env)->GetMethodID(env, cMP, "isPlaying", "()Z");
	jboolean r = JNI_FALSE;
	if (mIs) r = (*env)->CallBooleanMethod(env, g_player, mIs);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); r = JNI_FALSE; }
	(*env)->DeleteLocalRef(env, cMP);
	return r ? 1 : 0;
}
*/
import "C"

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	"fyne.io/fyne/v2/driver"
)

type audioPlayer struct {
	mu      sync.Mutex
	key     string
	onStop  func(key string)
	stopCh  chan struct{}
	running bool
}

func (p *audioPlayer) playing(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running && p.key == key
}

func (p *audioPlayer) start(key, path string, onStop func(key string)) error {
	p.stop()

	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	ok := false
	_ = driver.RunNative(func(ctx any) error {
		ac, valid := ctx.(*driver.AndroidContext)
		if !valid || ac == nil || ac.Env == 0 {
			return nil
		}
		ok = C.wt_play_start(C.uintptr_t(ac.Env), cPath) == 1
		return nil
	})
	if !ok {
		return fmt.Errorf("MediaPlayer failed to start")
	}

	stopCh := make(chan struct{})
	p.mu.Lock()
	p.key = key
	p.onStop = onStop
	p.stopCh = stopCh
	p.running = true
	p.mu.Unlock()

	go p.watch(key, stopCh)
	return nil
}

func (p *audioPlayer) watch(key string, stopCh chan struct{}) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			playing := false
			_ = driver.RunNative(func(ctx any) error {
				ac, valid := ctx.(*driver.AndroidContext)
				if !valid || ac == nil || ac.Env == 0 {
					return nil
				}
				playing = C.wt_play_is_playing(C.uintptr_t(ac.Env)) == 1
				return nil
			})
			if !playing {
				p.mu.Lock()
				if p.stopCh != stopCh {
					p.mu.Unlock()
					return
				}
				cb := p.onStop
				p.running = false
				p.key = ""
				p.onStop = nil
				p.stopCh = nil
				p.mu.Unlock()
				_ = driver.RunNative(func(ctx any) error {
					ac, valid := ctx.(*driver.AndroidContext)
					if !valid || ac == nil || ac.Env == 0 {
						return nil
					}
					C.wt_play_stop(C.uintptr_t(ac.Env))
					return nil
				})
				if cb != nil {
					cb(key)
				}
				return
			}
		}
	}
}

func (p *audioPlayer) stop() {
	p.mu.Lock()
	stopCh := p.stopCh
	cb := p.onStop
	key := p.key
	wasRunning := p.running
	p.running = false
	p.key = ""
	p.onStop = nil
	p.stopCh = nil
	p.mu.Unlock()

	if stopCh != nil {
		close(stopCh)
	}
	if wasRunning {
		_ = driver.RunNative(func(ctx any) error {
			ac, valid := ctx.(*driver.AndroidContext)
			if !valid || ac == nil || ac.Env == 0 {
				return nil
			}
			C.wt_play_stop(C.uintptr_t(ac.Env))
			return nil
		})
		if cb != nil {
			cb(key)
		}
	}
}
