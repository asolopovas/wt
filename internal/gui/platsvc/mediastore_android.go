//go:build android

package platsvc

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>


typedef struct {
	char** items;
	int    n;
	int    cap;
} wms_list;

static void wms_list_grow(wms_list* l) {
	int nc = l->cap == 0 ? 16 : l->cap * 2;
	char** ni = (char**)realloc(l->items, sizeof(char*) * nc);
	if (!ni) return;
	l->items = ni;
	l->cap = nc;
}

static void wms_list_push(wms_list* l, char* s) {
	if (l->n + 1 > l->cap) wms_list_grow(l);
	if (l->n + 1 > l->cap) { free(s); return; }
	l->items[l->n++] = s;
}

static int wms_query_audio(uintptr_t envPtr, uintptr_t actPtr, const char* prefix, wms_list* out) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject ctx = (jobject)actPtr;
	if (!env || !ctx || !prefix || !out) return -1;

	jclass cCtx = (*env)->GetObjectClass(env, ctx);
	jmethodID mGetCR = (*env)->GetMethodID(env, cCtx, "getContentResolver", "()Landroid/content/ContentResolver;");
	(*env)->DeleteLocalRef(env, cCtx);
	if (!mGetCR) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); return -1; }
	jobject cr = (*env)->CallObjectMethod(env, ctx, mGetCR);
	if (!cr) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); return -1; }

	jclass cMS = (*env)->FindClass(env, "android/provider/MediaStore$Audio$Media");
	if (!cMS) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, cr); return -1; }
	jfieldID fUri = (*env)->GetStaticFieldID(env, cMS, "EXTERNAL_CONTENT_URI", "Landroid/net/Uri;");
	jobject uri = (*env)->GetStaticObjectField(env, cMS, fUri);
	(*env)->DeleteLocalRef(env, cMS);
	if (!uri) { (*env)->DeleteLocalRef(env, cr); return -1; }

	jclass cString = (*env)->FindClass(env, "java/lang/String");
	jobjectArray proj = (*env)->NewObjectArray(env, 1, cString, NULL);
	jstring jData = (*env)->NewStringUTF(env, "_data");
	(*env)->SetObjectArrayElement(env, proj, 0, jData);
	(*env)->DeleteLocalRef(env, jData);

	jstring jSel = NULL;
	jobjectArray selArgs = NULL;
	(*env)->DeleteLocalRef(env, cString);
	(void)prefix;

	jclass cCR = (*env)->GetObjectClass(env, cr);
	jmethodID mQuery = (*env)->GetMethodID(env, cCR, "query",
		"(Landroid/net/Uri;[Ljava/lang/String;Ljava/lang/String;[Ljava/lang/String;Ljava/lang/String;)Landroid/database/Cursor;");
	(*env)->DeleteLocalRef(env, cCR);
	if (!mQuery) {
		if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
		(*env)->DeleteLocalRef(env, proj); (*env)->DeleteLocalRef(env, jSel);
		(*env)->DeleteLocalRef(env, selArgs); (*env)->DeleteLocalRef(env, uri); (*env)->DeleteLocalRef(env, cr);
		return -1;
	}

	jobject cursor = (*env)->CallObjectMethod(env, cr, mQuery, uri, proj, jSel, selArgs, NULL);
	(*env)->DeleteLocalRef(env, proj); (*env)->DeleteLocalRef(env, jSel);
	(*env)->DeleteLocalRef(env, selArgs); (*env)->DeleteLocalRef(env, uri); (*env)->DeleteLocalRef(env, cr);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return -1; }
	if (!cursor) return 0;

	jclass cCursor = (*env)->GetObjectClass(env, cursor);
	jmethodID mNext = (*env)->GetMethodID(env, cCursor, "moveToNext", "()Z");
	jmethodID mGetStr = (*env)->GetMethodID(env, cCursor, "getString", "(I)Ljava/lang/String;");
	jmethodID mClose = (*env)->GetMethodID(env, cCursor, "close", "()V");
	(*env)->DeleteLocalRef(env, cCursor);

	size_t plen = strlen(prefix);
	while ((*env)->CallBooleanMethod(env, cursor, mNext)) {
		jstring js = (jstring)(*env)->CallObjectMethod(env, cursor, mGetStr, (jint)0);
		if (!js) continue;
		const char* cs = (*env)->GetStringUTFChars(env, js, NULL);
		if (cs) {
			if (strncmp(cs, prefix, plen) == 0) {
				char* dup = strdup(cs);
				if (dup) wms_list_push(out, dup);
			}
			(*env)->ReleaseStringUTFChars(env, js, cs);
		}
		(*env)->DeleteLocalRef(env, js);
	}

	(*env)->CallVoidMethod(env, cursor, mClose);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
	(*env)->DeleteLocalRef(env, cursor);
	return 0;
}

static void wms_scan_path(uintptr_t envPtr, uintptr_t actPtr, const char* path) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject ctx = (jobject)actPtr;
	if (!env || !ctx || !path) return;

	jclass cIntent = (*env)->FindClass(env, "android/content/Intent");
	jmethodID mInit = (*env)->GetMethodID(env, cIntent, "<init>", "(Ljava/lang/String;)V");
	jstring jAction = (*env)->NewStringUTF(env, "android.intent.action.MEDIA_SCANNER_SCAN_FILE");
	jobject intent = (*env)->NewObject(env, cIntent, mInit, jAction);
	(*env)->DeleteLocalRef(env, jAction);

	jclass cUri = (*env)->FindClass(env, "android/net/Uri");
	jmethodID mFromFile = (*env)->GetStaticMethodID(env, cUri, "parse", "(Ljava/lang/String;)Landroid/net/Uri;");
	int plen = (int)strlen(path);
	char* full = (char*)malloc(plen + 8);
	memcpy(full, "file://", 7);
	memcpy(full + 7, path, plen + 1);
	jstring jURI = (*env)->NewStringUTF(env, full);
	free(full);
	jobject uri = (*env)->CallStaticObjectMethod(env, cUri, mFromFile, jURI);
	(*env)->DeleteLocalRef(env, jURI);
	(*env)->DeleteLocalRef(env, cUri);

	jmethodID mSetData = (*env)->GetMethodID(env, cIntent, "setData", "(Landroid/net/Uri;)Landroid/content/Intent;");
	jobject _r = (*env)->CallObjectMethod(env, intent, mSetData, uri);
	if (_r) (*env)->DeleteLocalRef(env, _r);
	(*env)->DeleteLocalRef(env, uri);

	jclass cCtx = (*env)->GetObjectClass(env, ctx);
	jmethodID mSend = (*env)->GetMethodID(env, cCtx, "sendBroadcast", "(Landroid/content/Intent;)V");
	(*env)->CallVoidMethod(env, ctx, mSend, intent);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
	(*env)->DeleteLocalRef(env, cCtx);
	(*env)->DeleteLocalRef(env, intent);
	(*env)->DeleteLocalRef(env, cIntent);
}

static void wms_scan_dir(uintptr_t envPtr, uintptr_t actPtr, const char* dir) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject ctx = (jobject)actPtr;
	if (!env || !ctx || !dir) return;

	jclass cMSc = (*env)->FindClass(env, "android/media/MediaScannerConnection");
	if (!cMSc) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); return; }
	jmethodID mScan = (*env)->GetStaticMethodID(env, cMSc, "scanFile",
		"(Landroid/content/Context;[Ljava/lang/String;[Ljava/lang/String;Landroid/media/MediaScannerConnection$OnScanCompletedListener;)V");
	if (!mScan) { if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, cMSc); return; }

	jclass cString = (*env)->FindClass(env, "java/lang/String");
	jobjectArray paths = (*env)->NewObjectArray(env, 1, cString, NULL);
	jstring jDir = (*env)->NewStringUTF(env, dir);
	(*env)->SetObjectArrayElement(env, paths, 0, jDir);
	(*env)->DeleteLocalRef(env, jDir);
	(*env)->DeleteLocalRef(env, cString);

	(*env)->CallStaticVoidMethod(env, cMSc, mScan, ctx, paths, NULL, NULL);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
	(*env)->DeleteLocalRef(env, paths);
	(*env)->DeleteLocalRef(env, cMSc);
}

static void wms_list_free(wms_list* l) {
	if (!l) return;
	for (int i = 0; i < l->n; i++) free(l->items[i]);
	free(l->items);
	l->items = NULL; l->n = 0; l->cap = 0;
}
*/
import "C"

import (
	"unsafe"

	"fyne.io/fyne/v2/driver"
)

func QueryAudioFilesIn(prefix string) []string {
	if prefix == "" {
		return nil
	}
	cPrefix := C.CString(prefix)
	defer C.free(unsafe.Pointer(cPrefix))
	var list C.wms_list
	defer C.wms_list_free(&list)
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wms_query_audio(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cPrefix, &list)
		return nil
	})
	if list.n <= 0 || list.items == nil {
		return nil
	}
	items := unsafe.Slice(list.items, int(list.n))
	out := make([]string, 0, len(items))
	for _, p := range items {
		if p == nil {
			continue
		}
		out = append(out, C.GoString(p))
	}
	return out
}

func RescanMediaPath(path string) {
	if path == "" {
		return
	}
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wms_scan_path(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cPath)
		return nil
	})
}

func RescanMediaDir(dir string) {
	if dir == "" {
		return
	}
	cDir := C.CString(dir)
	defer C.free(unsafe.Pointer(cDir))
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wms_scan_dir(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cDir)
		return nil
	})
}
