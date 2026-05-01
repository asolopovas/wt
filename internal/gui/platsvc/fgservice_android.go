//go:build android

package platsvc

/*
#cgo LDFLAGS: -llog

#include <jni.h>
#include <stdlib.h>
#include <android/log.h>

#define FG_TAG "wtfgsvc"
#define FG_LOGI(...) __android_log_print(ANDROID_LOG_INFO, FG_TAG, __VA_ARGS__)
#define FG_LOGE(...) __android_log_print(ANDROID_LOG_ERROR, FG_TAG, __VA_ARGS__)

// Loads an app class by name via the activity's ClassLoader. Required
// because FindClass from a Go-attached thread uses the system class loader,
// which cannot see classes packaged in the APK.
static jclass wt_fg_load_app_class(JNIEnv* env, jobject ctx, const char* dotted_name) {
	jclass cCtx = (*env)->GetObjectClass(env, ctx);
	jmethodID mGetCL = (*env)->GetMethodID(env, cCtx, "getClassLoader", "()Ljava/lang/ClassLoader;");
	(*env)->DeleteLocalRef(env, cCtx);
	if (!mGetCL) { (*env)->ExceptionClear(env); return NULL; }
	jobject cl = (*env)->CallObjectMethod(env, ctx, mGetCL);
	if ((*env)->ExceptionCheck(env) || !cl) { (*env)->ExceptionClear(env); return NULL; }
	jclass cCL = (*env)->GetObjectClass(env, cl);
	jmethodID mLoad = (*env)->GetMethodID(env, cCL, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
	(*env)->DeleteLocalRef(env, cCL);
	if (!mLoad) {
		(*env)->ExceptionClear(env);
		(*env)->DeleteLocalRef(env, cl);
		return NULL;
	}
	jstring jName = (*env)->NewStringUTF(env, dotted_name);
	jclass result = (jclass)(*env)->CallObjectMethod(env, cl, mLoad, jName);
	(*env)->DeleteLocalRef(env, jName);
	(*env)->DeleteLocalRef(env, cl);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return NULL; }
	return result;
}

static jobject wt_fg_make_intent(JNIEnv* env, jobject ctx) {
	jclass cIntent = (*env)->FindClass(env, "android/content/Intent");
	if (!cIntent) { (*env)->ExceptionClear(env); return NULL; }
	jclass cSvc = wt_fg_load_app_class(env, ctx, "com.asolopovas.wtranscribe.WtForegroundService");
	if (!cSvc) {
		(*env)->DeleteLocalRef(env, cIntent);
		FG_LOGE("WtForegroundService class not loadable via activity ClassLoader");
		return NULL;
	}
	jmethodID mInit = (*env)->GetMethodID(env, cIntent, "<init>", "(Landroid/content/Context;Ljava/lang/Class;)V");
	if (!mInit) {
		(*env)->ExceptionClear(env);
		(*env)->DeleteLocalRef(env, cIntent);
		(*env)->DeleteLocalRef(env, cSvc);
		return NULL;
	}
	jobject intent = (*env)->NewObject(env, cIntent, mInit, ctx, cSvc);
	(*env)->DeleteLocalRef(env, cIntent);
	(*env)->DeleteLocalRef(env, cSvc);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return NULL; }
	return intent;
}

static void wt_fg_start(uintptr_t env_p, uintptr_t ctx_p) {
	JNIEnv* env = (JNIEnv*)env_p;
	jobject ctx = (jobject)ctx_p;
	if (!env || !ctx) return;

	jobject intent = wt_fg_make_intent(env, ctx);
	if (!intent) return;

	jclass cCtx = (*env)->GetObjectClass(env, ctx);
	// API 26+: startForegroundService is required for backgrounded starts.
	jmethodID mStart = (*env)->GetMethodID(env, cCtx, "startForegroundService", "(Landroid/content/Intent;)Landroid/content/ComponentName;");
	if (!mStart) {
		(*env)->ExceptionClear(env);
		mStart = (*env)->GetMethodID(env, cCtx, "startService", "(Landroid/content/Intent;)Landroid/content/ComponentName;");
	}
	if (mStart) {
		jobject cn = (*env)->CallObjectMethod(env, ctx, mStart, intent);
		if ((*env)->ExceptionCheck(env)) {
			(*env)->ExceptionDescribe(env);
			(*env)->ExceptionClear(env);
			FG_LOGE("startForegroundService threw");
		} else {
			FG_LOGI("foreground service start requested");
		}
		if (cn) (*env)->DeleteLocalRef(env, cn);
	}
	(*env)->DeleteLocalRef(env, cCtx);
	(*env)->DeleteLocalRef(env, intent);
}

static void wt_fg_stop(uintptr_t env_p, uintptr_t ctx_p) {
	JNIEnv* env = (JNIEnv*)env_p;
	jobject ctx = (jobject)ctx_p;
	if (!env || !ctx) return;

	jobject intent = wt_fg_make_intent(env, ctx);
	if (!intent) return;

	jclass cCtx = (*env)->GetObjectClass(env, ctx);
	jmethodID mStop = (*env)->GetMethodID(env, cCtx, "stopService", "(Landroid/content/Intent;)Z");
	if (mStop) {
		(*env)->CallBooleanMethod(env, ctx, mStop, intent);
		if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); }
		FG_LOGI("foreground service stop requested");
	}
	(*env)->DeleteLocalRef(env, cCtx);
	(*env)->DeleteLocalRef(env, intent);
}
*/
import "C"

import (
	"fyne.io/fyne/v2/driver"
)

func StartForegroundService() {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wt_fg_start(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
		return nil
	})
}

func StopForegroundService() {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wt_fg_stop(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
		return nil
	})
}
