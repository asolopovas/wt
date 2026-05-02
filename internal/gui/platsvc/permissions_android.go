//go:build android

package platsvc

/*
#cgo LDFLAGS: -llog

#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include <android/log.h>

#define WP_TAG "wtperm"
#define WP_LOGI(...) __android_log_print(ANDROID_LOG_INFO, WP_TAG, __VA_ARGS__)
#define WP_LOGE(...) __android_log_print(ANDROID_LOG_ERROR, WP_TAG, __VA_ARGS__)

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
	return res == 0 ? 1 : 0;
}

static int wt_should_show_rationale(uintptr_t envPtr, uintptr_t actPtr, const char* perm) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject act = (jobject)actPtr;
	if (!env || !act || !perm) return 0;

	jclass cAct = (*env)->GetObjectClass(env, act);
	jmethodID mShow = (*env)->GetMethodID(env, cAct, "shouldShowRequestPermissionRationale", "(Ljava/lang/String;)Z");
	(*env)->DeleteLocalRef(env, cAct);
	if (!mShow) {
		if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
		return 0;
	}
	jstring jPerm = (*env)->NewStringUTF(env, perm);
	jboolean res = (*env)->CallBooleanMethod(env, act, mShow, jPerm);
	(*env)->DeleteLocalRef(env, jPerm);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return 0; }
	return res ? 1 : 0;
}

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
	jobject _r2 = (*env)->CallObjectMethod(env, intent, mAddFlags, (jint)0x10000000);
	if (_r2) (*env)->DeleteLocalRef(env, _r2);

	jmethodID mStart = (*env)->GetMethodID(env, cAct, "startActivity", "(Landroid/content/Intent;)V");
	(*env)->CallVoidMethod(env, act, mStart, intent);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); }

	(*env)->DeleteLocalRef(env, cAct);
	(*env)->DeleteLocalRef(env, intent);
	(*env)->DeleteLocalRef(env, cIntent);
}

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

static void wt_open_battery_settings(uintptr_t envPtr, uintptr_t actPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject act = (jobject)actPtr;
	if (!env || !act) return;

	if (wt_start_action_intent(env, act, "android.settings.IGNORE_BATTERY_OPTIMIZATION_SETTINGS")) return;
	wt_start_action_intent(env, act, "android.settings.SETTINGS");
}

static void wt_set_thread_priority(uintptr_t envPtr, int tid, int prio) {
	JNIEnv* env = (JNIEnv*)envPtr;
	if (!env) return;
	jclass cProc = (*env)->FindClass(env, "android/os/Process");
	if (!cProc) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); return; }
	jmethodID mSet = (*env)->GetStaticMethodID(env, cProc, "setThreadPriority", "(II)V");
	if (!mSet) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, cProc); return; }
	(*env)->CallStaticVoidMethod(env, cProc, mSet, (jint)tid, (jint)prio);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
	(*env)->DeleteLocalRef(env, cProc);
}

static int wt_is_screen_on(uintptr_t envPtr, uintptr_t actPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject act = (jobject)actPtr;
	if (!env || !act) return 1;

	jclass cAct = (*env)->GetObjectClass(env, act);
	jmethodID mGetSvc = (*env)->GetMethodID(env, cAct, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;");
	jstring jSvc = (*env)->NewStringUTF(env, "power");
	jobject pm = (*env)->CallObjectMethod(env, act, mGetSvc, jSvc);
	(*env)->DeleteLocalRef(env, jSvc);
	(*env)->DeleteLocalRef(env, cAct);
	if (!pm) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); return 1; }

	jclass cPM = (*env)->GetObjectClass(env, pm);
	jmethodID mIs = (*env)->GetMethodID(env, cPM, "isInteractive", "()Z");
	int res = 1;
	if (mIs) {
		jboolean b = (*env)->CallBooleanMethod(env, pm, mIs);
		res = b ? 1 : 0;
	} else if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
	(*env)->DeleteLocalRef(env, cPM);
	(*env)->DeleteLocalRef(env, pm);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); }
	return res;
}

static int wt_is_external_storage_manager(uintptr_t envPtr, uintptr_t actPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject act = (jobject)actPtr;
	if (!env || !act) return 0;
	jclass cEnv = (*env)->FindClass(env, "android/os/Environment");
	if (!cEnv) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); return 0; }
	jmethodID mIs = (*env)->GetStaticMethodID(env, cEnv, "isExternalStorageManager", "()Z");
	if (!mIs) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, cEnv); return 0; }
	jboolean res = (*env)->CallStaticBooleanMethod(env, cEnv, mIs);
	(*env)->DeleteLocalRef(env, cEnv);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return 0; }
	return res ? 1 : 0;
}

static void wt_open_all_files_access(uintptr_t envPtr, uintptr_t actPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject act = (jobject)actPtr;
	if (!env || !act) return;

	jclass cIntent = (*env)->FindClass(env, "android/content/Intent");
	jmethodID mInit = (*env)->GetMethodID(env, cIntent, "<init>", "(Ljava/lang/String;)V");
	jstring jAction = (*env)->NewStringUTF(env, "android.settings.MANAGE_APP_ALL_FILES_ACCESS_PERMISSION");
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
	jobject _r2 = (*env)->CallObjectMethod(env, intent, mAddFlags, (jint)0x10000000);
	if (_r2) (*env)->DeleteLocalRef(env, _r2);

	jmethodID mStart = (*env)->GetMethodID(env, cAct, "startActivity", "(Landroid/content/Intent;)V");
	(*env)->CallVoidMethod(env, act, mStart, intent);
	int failed = (*env)->ExceptionCheck(env);
	if (failed) { (*env)->ExceptionClear(env); }

	(*env)->DeleteLocalRef(env, cAct);
	(*env)->DeleteLocalRef(env, intent);
	(*env)->DeleteLocalRef(env, cIntent);

	if (failed) {
		wt_start_action_intent(env, act, "android.settings.MANAGE_ALL_FILES_ACCESS_PERMISSION");
	}
}

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
	PermRecordAudio    = "android.permission.RECORD_AUDIO"
	PermReadMediaAudio = "android.permission.READ_MEDIA_AUDIO"
	PermReadStorage    = "android.permission.READ_EXTERNAL_STORAGE"
	PermPostNotif      = "android.permission.POST_NOTIFICATIONS"
)

type PermissionInfo struct {
	ID      string
	Label   string
	Purpose string
	Granted bool
}

func runtimePermissions() []string {
	if AndroidSDKInt() >= 33 {
		return []string{PermRecordAudio, PermReadMediaAudio, PermPostNotif}
	}
	return []string{PermRecordAudio, PermReadStorage}
}

func permissionLabel(id string) (string, string) {
	switch id {
	case PermRecordAudio:
		return "MICROPHONE", "Record audio."
	case PermReadMediaAudio:
		return "AUDIO FILES", "Import audio."
	case PermReadStorage:
		return "STORAGE", "Import audio."
	case PermPostNotif:
		return "NOTIFICATIONS", "Background progress."
	}
	return id, ""
}

func CheckPermission(id string) bool {
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

func ShouldShowPermissionRationale(id string) bool {
	out := false
	cID := C.CString(id)
	defer C.free(unsafe.Pointer(cID))
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		out = C.wt_should_show_rationale(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cID) == 1
		return nil
	})
	return out
}

func RequestPermissions(ids []string) {
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

func OpenAppSettings() {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wt_open_app_settings(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
		return nil
	})
}

func OpenBatteryOptimizationSettings() {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wt_open_battery_settings(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
		return nil
	})
}

func IsScreenInteractive() bool {
	out := true
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		out = C.wt_is_screen_on(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx)) == 1
		return nil
	})
	return out
}

const ThreadPriorityBackground = 10

func SetThreadPriorityViaProcess(tid int) {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 {
			return nil
		}
		C.wt_set_thread_priority(C.uintptr_t(ac.Env), C.int(tid), C.int(ThreadPriorityBackground))
		return nil
	})
}

func IsExternalStorageManager() bool {
	if AndroidSDKInt() < 30 {
		return true
	}
	out := false
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		out = C.wt_is_external_storage_manager(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx)) == 1
		return nil
	})
	return out
}

func OpenAllFilesAccessSettings() {
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wt_open_all_files_access(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
		return nil
	})
}

func IsIgnoringBatteryOptimizations() bool {
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

func CollectPermissionInfos() []PermissionInfo {
	ids := runtimePermissions()
	out := make([]PermissionInfo, 0, len(ids))
	for _, id := range ids {
		label, purpose := permissionLabel(id)
		out = append(out, PermissionInfo{
			ID:      id,
			Label:   label,
			Purpose: purpose,
			Granted: CheckPermission(id),
		})
	}
	return out
}

func MissingPermissions() []string {
	var out []string
	for _, p := range CollectPermissionInfos() {
		if !p.Granted {
			out = append(out, p.ID)
		}
	}
	return out
}
