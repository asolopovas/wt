//go:build android

package platsvc

/*
#cgo LDFLAGS: -llog

#include <jni.h>
#include <stdlib.h>
#include <android/log.h>

#define KS_TAG "wtkeepscreen"
#define KS_LOGI(...) __android_log_print(ANDROID_LOG_INFO, KS_TAG, __VA_ARGS__)

// FLAG_KEEP_SCREEN_ON = 0x00000080
#define WT_FLAG_KEEP_SCREEN_ON 0x00000080

static void wt_keep_screen(uintptr_t env_p, uintptr_t ctx_p, int on) {
	JNIEnv* env = (JNIEnv*)env_p;
	jobject activity = (jobject)ctx_p;
	if (!env || !activity) return;

	jclass cAct = (*env)->GetObjectClass(env, activity);
	jmethodID mGetWin = (*env)->GetMethodID(env, cAct, "getWindow", "()Landroid/view/Window;");
	(*env)->DeleteLocalRef(env, cAct);
	if (!mGetWin) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); return; }

	jobject win = (*env)->CallObjectMethod(env, activity, mGetWin);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return; }
	if (!win) return;

	jclass cWin = (*env)->GetObjectClass(env, win);
	const char* method = on ? "addFlags" : "clearFlags";
	jmethodID mFlag = (*env)->GetMethodID(env, cWin, method, "(I)V");
	if (mFlag) {
		(*env)->CallVoidMethod(env, win, mFlag, (jint)WT_FLAG_KEEP_SCREEN_ON);
		if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
		KS_LOGI("keep-screen-on=%d", on);
	}
	(*env)->DeleteLocalRef(env, cWin);
	(*env)->DeleteLocalRef(env, win);
}
*/
import "C"

import (
	"fyne.io/fyne/v2/driver"
)

func setKeepScreenOn(on bool) {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		v := 0
		if on {
			v = 1
		}
		C.wt_keep_screen(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), C.int(v))
		return nil
	})
}

func KeepScreenOn()    { setKeepScreenOn(true) }
func ReleaseScreenOn() { setKeepScreenOn(false) }
