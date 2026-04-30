//go:build android

package gui

/*
#cgo LDFLAGS: -llog

#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include <android/log.h>

#define WP_TAG "wtperm"
#define WP_LOGI(...) __android_log_print(ANDROID_LOG_INFO, WP_TAG, __VA_ARGS__)
#define WP_LOGE(...) __android_log_print(ANDROID_LOG_ERROR, WP_TAG, __VA_ARGS__)

// wt_check_permission returns 1 if ContextCompat.checkSelfPermission == PERMISSION_GRANTED.
static int wt_check_permission(uintptr_t envPtr, uintptr_t actPtr, const char* perm) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject ctx = (jobject)actPtr;
	if (!env || !ctx || !perm) return 0;

	jclass cCtx = (*env)->GetObjectClass(env, ctx);
	jmethodID mCheck = (*env)->GetMethodID(env, cCtx, "checkSelfPermission", "(Ljava/lang/String;)I");
	(*env)->DeleteLocalRef(env, cCtx);
	if (!mCheck) {
		if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
		return 0;
	}

	jstring jPerm = (*env)->NewStringUTF(env, perm);
	jint res = (*env)->CallIntMethod(env, ctx, mCheck, jPerm);
	(*env)->DeleteLocalRef(env, jPerm);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return 0; }
	// PackageManager.PERMISSION_GRANTED == 0
	return res == 0 ? 1 : 0;
}

// wt_request_permissions calls Activity.requestPermissions(perms[], 0).
static void wt_request_permissions(uintptr_t envPtr, uintptr_t actPtr, const char** perms, int n) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject act = (jobject)actPtr;
	if (!env || !act || n <= 0) return;

	jclass cAct = (*env)->GetObjectClass(env, act);
	jmethodID mReq = (*env)->GetMethodID(env, cAct, "requestPermissions", "([Ljava/lang/String;I)V");
	(*env)->DeleteLocalRef(env, cAct);
	if (!mReq) {
		if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
		return;
	}

	jclass cString = (*env)->FindClass(env, "java/lang/String");
	jobjectArray arr = (*env)->NewObjectArray(env, n, cString, NULL);
	for (int i = 0; i < n; i++) {
		jstring js = (*env)->NewStringUTF(env, perms[i]);
		(*env)->SetObjectArrayElement(env, arr, i, js);
		(*env)->DeleteLocalRef(env, js);
	}

	(*env)->CallVoidMethod(env, act, mReq, arr, (jint)0);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); }
	(*env)->DeleteLocalRef(env, arr);
	(*env)->DeleteLocalRef(env, cString);
}

// wt_open_app_settings launches Settings.ACTION_APPLICATION_DETAILS_SETTINGS for our package.
static void wt_open_app_settings(uintptr_t envPtr, uintptr_t actPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject act = (jobject)actPtr;
	if (!env || !act) return;

	jclass cIntent = (*env)->FindClass(env, "android/content/Intent");
	jmethodID mInit = (*env)->GetMethodID(env, cIntent, "<init>", "(Ljava/lang/String;)V");
	jstring jAction = (*env)->NewStringUTF(env, "android.settings.APPLICATION_DETAILS_SETTINGS");
	jobject intent = (*env)->NewObject(env, cIntent, mInit, jAction);
	(*env)->DeleteLocalRef(env, jAction);

	jclass cAct = (*env)->GetObjectClass(env, act);
	jmethodID mGetPkg = (*env)->GetMethodID(env, cAct, "getPackageName", "()Ljava/lang/String;");
	jstring jPkg = (jstring)(*env)->CallObjectMethod(env, act, mGetPkg);

	jclass cUri = (*env)->FindClass(env, "android/net/Uri");
	jmethodID mFromParts = (*env)->GetStaticMethodID(env, cUri, "fromParts",
		"(Ljava/lang/String;Ljava/lang/String;Ljava/lang/String;)Landroid/net/Uri;");
	jstring jScheme = (*env)->NewStringUTF(env, "package");
	jobject uri = (*env)->CallStaticObjectMethod(env, cUri, mFromParts, jScheme, jPkg, NULL);
	(*env)->DeleteLocalRef(env, jScheme);
	(*env)->DeleteLocalRef(env, jPkg);
	(*env)->DeleteLocalRef(env, cUri);

	jmethodID mSetData = (*env)->GetMethodID(env, cIntent, "setData", "(Landroid/net/Uri;)Landroid/content/Intent;");
	jobject _r = (*env)->CallObjectMethod(env, intent, mSetData, uri);
	if (_r) (*env)->DeleteLocalRef(env, _r);
	(*env)->DeleteLocalRef(env, uri);

	jmethodID mAddFlags = (*env)->GetMethodID(env, cIntent, "addFlags", "(I)Landroid/content/Intent;");
	// FLAG_ACTIVITY_NEW_TASK = 0x10000000
	jobject _r2 = (*env)->CallObjectMethod(env, intent, mAddFlags, (jint)0x10000000);
	if (_r2) (*env)->DeleteLocalRef(env, _r2);

	jmethodID mStart = (*env)->GetMethodID(env, cAct, "startActivity", "(Landroid/content/Intent;)V");
	(*env)->CallVoidMethod(env, act, mStart, intent);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); }

	(*env)->DeleteLocalRef(env, cAct);
	(*env)->DeleteLocalRef(env, intent);
	(*env)->DeleteLocalRef(env, cIntent);
}

// wt_start_action_intent launches Intent(action) with FLAG_ACTIVITY_NEW_TASK.
// Returns 1 on success.
static int wt_start_action_intent(JNIEnv* env, jobject act, const char* action) {
	jclass cIntent = (*env)->FindClass(env, "android/content/Intent");
	jmethodID mInit = (*env)->GetMethodID(env, cIntent, "<init>", "(Ljava/lang/String;)V");
	jstring jAction = (*env)->NewStringUTF(env, action);
	jobject intent = (*env)->NewObject(env, cIntent, mInit, jAction);
	(*env)->DeleteLocalRef(env, jAction);

	jmethodID mAddFlags = (*env)->GetMethodID(env, cIntent, "addFlags", "(I)Landroid/content/Intent;");
	jobject _r = (*env)->CallObjectMethod(env, intent, mAddFlags, (jint)0x10000000);
	if (_r) (*env)->DeleteLocalRef(env, _r);

	jclass cAct = (*env)->GetObjectClass(env, act);
	jmethodID mStart = (*env)->GetMethodID(env, cAct, "startActivity", "(Landroid/content/Intent;)V");
	(*env)->CallVoidMethod(env, act, mStart, intent);
	int ok = 1;
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); ok = 0; }

	(*env)->DeleteLocalRef(env, cAct);
	(*env)->DeleteLocalRef(env, intent);
	(*env)->DeleteLocalRef(env, cIntent);
	return ok;
}

// wt_open_battery_settings opens the system list of battery-optimized apps.
// ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS works on all Android versions
// without the package URI (REQUEST_IGNORE_BATTERY_OPTIMIZATIONS is restricted
// by Google Play policy and silently fails).
static void wt_open_battery_settings(uintptr_t envPtr, uintptr_t actPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject act = (jobject)actPtr;
	if (!env || !act) return;

	if (wt_start_action_intent(env, act, "android.settings.IGNORE_BATTERY_OPTIMIZATION_SETTINGS")) return;
	wt_start_action_intent(env, act, "android.settings.SETTINGS");
}

// wt_is_ignoring_battery returns 1 if our package is whitelisted from battery optimizations.
static int wt_is_ignoring_battery(uintptr_t envPtr, uintptr_t actPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject act = (jobject)actPtr;
	if (!env || !act) return 0;

	jclass cAct = (*env)->GetObjectClass(env, act);
	jmethodID mGetSvc = (*env)->GetMethodID(env, cAct, "getSystemService",
		"(Ljava/lang/String;)Ljava/lang/Object;");
	jmethodID mGetPkg = (*env)->GetMethodID(env, cAct, "getPackageName", "()Ljava/lang/String;");
	jstring jSvc = (*env)->NewStringUTF(env, "power");
	jobject pm = (*env)->CallObjectMethod(env, act, mGetSvc, jSvc);
	(*env)->DeleteLocalRef(env, jSvc);
	(*env)->DeleteLocalRef(env, cAct);
	if (!pm) {
		if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
		return 0;
	}

	jclass cPM = (*env)->GetObjectClass(env, pm);
	jmethodID mIgn = (*env)->GetMethodID(env, cPM, "isIgnoringBatteryOptimizations",
		"(Ljava/lang/String;)Z");
	if (!mIgn) {
		if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
		(*env)->DeleteLocalRef(env, cPM);
		(*env)->DeleteLocalRef(env, pm);
		return 0;
	}
	jstring jPkg = (jstring)(*env)->CallObjectMethod(env, act,
		(*env)->GetMethodID(env, (*env)->GetObjectClass(env, act), "getPackageName", "()Ljava/lang/String;"));
	(void)mGetPkg;
	jboolean ok = (*env)->CallBooleanMethod(env, pm, mIgn, jPkg);
	(*env)->DeleteLocalRef(env, jPkg);
	(*env)->DeleteLocalRef(env, cPM);
	(*env)->DeleteLocalRef(env, pm);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return 0; }
	return ok ? 1 : 0;
}
*/
import "C"

import (
	"unsafe"

	"fyne.io/fyne/v2/driver"
)

const (
	permRecordAudio    = "android.permission.RECORD_AUDIO"
	permReadMediaAudio = "android.permission.READ_MEDIA_AUDIO"
	permReadStorage    = "android.permission.READ_EXTERNAL_STORAGE"
	permPostNotif      = "android.permission.POST_NOTIFICATIONS"
)

type permissionInfo struct {
	id      string
	label   string
	purpose string
	granted bool
}

func runtimePermissions() []string {
	if androidSDKInt() >= 33 {
		return []string{permRecordAudio, permReadMediaAudio, permPostNotif}
	}
	return []string{permRecordAudio, permReadStorage}
}

func permissionLabel(id string) (string, string) {
	switch id {
	case permRecordAudio:
		return "MICROPHONE", "Record audio."
	case permReadMediaAudio:
		return "AUDIO FILES", "Import audio."
	case permReadStorage:
		return "STORAGE", "Import audio."
	case permPostNotif:
		return "NOTIFICATIONS", "Background progress."
	}
	return id, ""
}

func checkPermission(id string) bool {
	granted := false
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		granted = C.wt_check_permission(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cID) == 1
		return nil
	})
	return granted
}

func requestPermissions(ids []string) {
	if len(ids) == 0 {
		return
	}
	cArr := make([]*C.char, len(ids))
	for i, id := range ids {
		cArr[i] = C.CString(id)
	}
	defer func() {
		for _, c := range cArr {
			C.free(unsafe.Pointer(c))
		}
	}()
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wt_request_permissions(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx),
			(**C.char)(unsafe.Pointer(&cArr[0])), C.int(len(cArr)))
		return nil
	})
}

func openAppSettings() {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wt_open_app_settings(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
		return nil
	})
}

func openBatteryOptimizationSettings() {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wt_open_battery_settings(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
		return nil
	})
}

func isIgnoringBatteryOptimizations() bool {
	out := false
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		out = C.wt_is_ignoring_battery(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx)) == 1
		return nil
	})
	return out
}

func collectPermissionInfos() []permissionInfo {
	ids := runtimePermissions()
	out := make([]permissionInfo, 0, len(ids))
	for _, id := range ids {
		label, purpose := permissionLabel(id)
		out = append(out, permissionInfo{
			id:      id,
			label:   label,
			purpose: purpose,
			granted: checkPermission(id),
		})
	}
	return out
}

func missingPermissions() []string {
	var out []string
	for _, p := range collectPermissionInfos() {
		if !p.granted {
			out = append(out, p.id)
		}
	}
	return out
}
