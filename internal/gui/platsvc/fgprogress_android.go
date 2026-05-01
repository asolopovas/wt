//go:build android

package platsvc

/*
#cgo LDFLAGS: -llog

#include <jni.h>
#include <stdlib.h>
#include <android/log.h>

#define FP_TAG "wtfgprog"
#define FP_LOGE(...) __android_log_print(ANDROID_LOG_ERROR, FP_TAG, __VA_ARGS__)

static jclass wt_fp_load_app_class(JNIEnv* env, jobject ctx, const char* dotted) {
	jclass cCtx = (*env)->GetObjectClass(env, ctx);
	jmethodID mGetCL = (*env)->GetMethodID(env, cCtx, "getClassLoader", "()Ljava/lang/ClassLoader;");
	(*env)->DeleteLocalRef(env, cCtx);
	if (!mGetCL) { (*env)->ExceptionClear(env); return NULL; }
	jobject cl = (*env)->CallObjectMethod(env, ctx, mGetCL);
	if ((*env)->ExceptionCheck(env) || !cl) { (*env)->ExceptionClear(env); return NULL; }
	jclass cCL = (*env)->GetObjectClass(env, cl);
	jmethodID mLoad = (*env)->GetMethodID(env, cCL, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
	(*env)->DeleteLocalRef(env, cCL);
	if (!mLoad) { (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, cl); return NULL; }
	jstring jName = (*env)->NewStringUTF(env, dotted);
	jclass result = (jclass)(*env)->CallObjectMethod(env, cl, mLoad, jName);
	(*env)->DeleteLocalRef(env, jName);
	(*env)->DeleteLocalRef(env, cl);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return NULL; }
	return result;
}

static void wt_fp_update(uintptr_t env_p, uintptr_t ctx_p, int percent, const char* text) {
	JNIEnv* env = (JNIEnv*)env_p;
	jobject ctx = (jobject)ctx_p;
	if (!env || !ctx) return;

	jclass cSvc = wt_fp_load_app_class(env, ctx, "com.asolopovas.wtranscribe.WtForegroundService");
	if (!cSvc) { FP_LOGE("class not loadable"); return; }

	jmethodID mUpd = (*env)->GetStaticMethodID(env, cSvc, "updateProgress", "(ILjava/lang/String;)V");
	if (!mUpd) { (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, cSvc); FP_LOGE("updateProgress not found"); return; }

	jstring jText = (*env)->NewStringUTF(env, text ? text : "");
	(*env)->CallStaticVoidMethod(env, cSvc, mUpd, (jint)percent, jText);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); }
	(*env)->DeleteLocalRef(env, jText);
	(*env)->DeleteLocalRef(env, cSvc);
}
*/
import "C"

import (
	"unsafe"

	"fyne.io/fyne/v2/driver"
)

func UpdateProgress(percent int, text string) {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		ct := C.CString(text)
		defer C.free(unsafe.Pointer(ct))
		C.wt_fp_update(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), C.int(percent), ct)
		return nil
	})
}
