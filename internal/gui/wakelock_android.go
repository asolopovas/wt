//go:build android

package gui

/*
#cgo LDFLAGS: -llog

#include <jni.h>
#include <stdlib.h>
#include <android/log.h>

#define WL_TAG "wtwake"
#define WL_LOGI(...) __android_log_print(ANDROID_LOG_INFO, WL_TAG, __VA_ARGS__)
#define WL_LOGE(...) __android_log_print(ANDROID_LOG_ERROR, WL_TAG, __VA_ARGS__)

static jobject g_wake_lock = NULL;

static void wt_wake_acquire(uintptr_t env_p, uintptr_t ctx_p) {
	JNIEnv* env = (JNIEnv*)env_p;
	jobject ctx = (jobject)ctx_p;
	if (!env || !ctx) return;
	if (g_wake_lock != NULL) return;

	jclass cCtx = (*env)->GetObjectClass(env, ctx);
	jmethodID mGetSvc = (*env)->GetMethodID(env, cCtx, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;");
	(*env)->DeleteLocalRef(env, cCtx);
	if (!mGetSvc) return;

	jstring jSvc = (*env)->NewStringUTF(env, "power");
	jobject pm = (*env)->CallObjectMethod(env, ctx, mGetSvc, jSvc);
	(*env)->DeleteLocalRef(env, jSvc);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return; }
	if (!pm) return;

	jclass cPM = (*env)->GetObjectClass(env, pm);
	jmethodID mNewWL = (*env)->GetMethodID(env, cPM, "newWakeLock", "(ILjava/lang/String;)Landroid/os/PowerManager$WakeLock;");
	if (!mNewWL) { (*env)->DeleteLocalRef(env, cPM); (*env)->DeleteLocalRef(env, pm); return; }

	jstring jTag = (*env)->NewStringUTF(env, "wt:transcribe");
	// PARTIAL_WAKE_LOCK = 1
	jobject wl = (*env)->CallObjectMethod(env, pm, mNewWL, (jint)1, jTag);
	(*env)->DeleteLocalRef(env, jTag);
	(*env)->DeleteLocalRef(env, cPM);
	(*env)->DeleteLocalRef(env, pm);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return; }
	if (!wl) return;

	jclass cWL = (*env)->GetObjectClass(env, wl);
	jmethodID mSetRef = (*env)->GetMethodID(env, cWL, "setReferenceCounted", "(Z)V");
	if (mSetRef) (*env)->CallVoidMethod(env, wl, mSetRef, JNI_FALSE);
	jmethodID mAcq = (*env)->GetMethodID(env, cWL, "acquire", "()V");
	if (mAcq) (*env)->CallVoidMethod(env, wl, mAcq);
	(*env)->DeleteLocalRef(env, cWL);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); }

	g_wake_lock = (*env)->NewGlobalRef(env, wl);
	(*env)->DeleteLocalRef(env, wl);
	WL_LOGI("wake lock acquired");
}

static void wt_wake_release(uintptr_t env_p) {
	JNIEnv* env = (JNIEnv*)env_p;
	if (!env || g_wake_lock == NULL) return;

	jclass cWL = (*env)->GetObjectClass(env, g_wake_lock);
	jmethodID mIsHeld = (*env)->GetMethodID(env, cWL, "isHeld", "()Z");
	jmethodID mRel = (*env)->GetMethodID(env, cWL, "release", "()V");
	if (mIsHeld && mRel) {
		jboolean held = (*env)->CallBooleanMethod(env, g_wake_lock, mIsHeld);
		if (held) (*env)->CallVoidMethod(env, g_wake_lock, mRel);
	}
	(*env)->DeleteLocalRef(env, cWL);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); }

	(*env)->DeleteGlobalRef(env, g_wake_lock);
	g_wake_lock = NULL;
	WL_LOGI("wake lock released");
}
*/
import "C"

import (
	"fyne.io/fyne/v2/driver"
)

func acquireWakeLock() {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wt_wake_acquire(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
		return nil
	})
}

func releaseWakeLock() {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 {
			return nil
		}
		C.wt_wake_release(C.uintptr_t(ac.Env))
		return nil
	})
}
