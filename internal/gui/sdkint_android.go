//go:build android

package gui

/*
#include <jni.h>

static int wt_sdk_int(uintptr_t envPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	if (!env) return 0;
	jclass cVer = (*env)->FindClass(env, "android/os/Build$VERSION");
	if (!cVer) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); return 0; }
	jfieldID fSDK = (*env)->GetStaticFieldID(env, cVer, "SDK_INT", "I");
	if (!fSDK) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, cVer); return 0; }
	jint v = (*env)->GetStaticIntField(env, cVer, fSDK);
	(*env)->DeleteLocalRef(env, cVer);
	return (int)v;
}
*/
import "C"

import (
	"sync/atomic"

	"fyne.io/fyne/v2/driver"
)

var cachedSDKInt atomic.Int32

func androidSDKInt() int {
	if v := cachedSDKInt.Load(); v > 0 {
		return int(v)
	}
	out := 0
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 {
			return nil
		}
		out = int(C.wt_sdk_int(C.uintptr_t(ac.Env)))
		return nil
	})
	if out > 0 {
		cachedSDKInt.Store(int32(out))
	}
	return out
}
