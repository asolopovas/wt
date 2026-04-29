//go:build android

package gui

/*
#cgo LDFLAGS: -llog

#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <android/log.h>

#define WT_TAG "wtshare"
#define WT_LOGI(...) __android_log_print(ANDROID_LOG_INFO, WT_TAG, __VA_ARGS__)
#define WT_LOGE(...) __android_log_print(ANDROID_LOG_ERROR, WT_TAG, __VA_ARGS__)

// wt_share_result is a simple C-side container for paths produced by an intent intake.
typedef struct {
	char** paths;
	int    count;
	int    capacity;
} wt_share_result;

static void wt_result_push(wt_share_result* r, const char* p) {
	if (r->count >= r->capacity) {
		int nc = r->capacity == 0 ? 4 : r->capacity * 2;
		char** np = (char**)realloc(r->paths, sizeof(char*) * nc);
		if (!np) return;
		r->paths = np;
		r->capacity = nc;
	}
	r->paths[r->count++] = strdup(p);
}

// wt_jstring_to_c copies a jstring into a malloc'd C string.
static char* wt_jstring_to_c(JNIEnv* env, jstring s) {
	if (!s) return NULL;
	const char* utf = (*env)->GetStringUTFChars(env, s, NULL);
	if (!utf) return NULL;
	char* out = strdup(utf);
	(*env)->ReleaseStringUTFChars(env, s, utf);
	return out;
}

// wt_query_display_name calls resolver.query(uri, [DISPLAY_NAME], null, null, null)
// and returns a malloc'd string for the OpenableColumns.DISPLAY_NAME, or NULL.
static char* wt_query_display_name(JNIEnv* env, jobject resolver, jobject uri) {
	jclass cResolver = (*env)->GetObjectClass(env, resolver);
	jmethodID mQuery = (*env)->GetMethodID(env, cResolver,
		"query",
		"(Landroid/net/Uri;[Ljava/lang/String;Ljava/lang/String;[Ljava/lang/String;Ljava/lang/String;)Landroid/database/Cursor;");
	if (!mQuery) { (*env)->DeleteLocalRef(env, cResolver); return NULL; }

	jstring jDisplay = (*env)->NewStringUTF(env, "_display_name");
	jclass cString = (*env)->FindClass(env, "java/lang/String");
	jobjectArray proj = (*env)->NewObjectArray(env, 1, cString, jDisplay);

	jobject cursor = (*env)->CallObjectMethod(env, resolver, mQuery, uri, proj, NULL, NULL, NULL);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); cursor = NULL; }

	char* out = NULL;
	if (cursor) {
		jclass cCursor = (*env)->GetObjectClass(env, cursor);
		jmethodID mMoveFirst = (*env)->GetMethodID(env, cCursor, "moveToFirst", "()Z");
		jmethodID mGetIndex  = (*env)->GetMethodID(env, cCursor, "getColumnIndex", "(Ljava/lang/String;)I");
		jmethodID mGetString = (*env)->GetMethodID(env, cCursor, "getString", "(I)Ljava/lang/String;");
		jmethodID mClose     = (*env)->GetMethodID(env, cCursor, "close", "()V");
		if ((*env)->CallBooleanMethod(env, cursor, mMoveFirst)) {
			jint idx = (*env)->CallIntMethod(env, cursor, mGetIndex, jDisplay);
			if (idx >= 0) {
				jstring js = (jstring)(*env)->CallObjectMethod(env, cursor, mGetString, idx);
				if (js) {
					out = wt_jstring_to_c(env, js);
					(*env)->DeleteLocalRef(env, js);
				}
			}
		}
		(*env)->CallVoidMethod(env, cursor, mClose);
		(*env)->DeleteLocalRef(env, cCursor);
		(*env)->DeleteLocalRef(env, cursor);
	}

	(*env)->DeleteLocalRef(env, proj);
	(*env)->DeleteLocalRef(env, cString);
	(*env)->DeleteLocalRef(env, jDisplay);
	(*env)->DeleteLocalRef(env, cResolver);
	return out;
}

// wt_unique_path returns a malloc'd path under outDir, suffixing -N if needed to avoid collisions.
static char* wt_unique_path(const char* outDir, const char* name) {
	char buf[1024];
	snprintf(buf, sizeof(buf), "%s/%s", outDir, name);
	FILE* f = fopen(buf, "rb");
	if (!f) return strdup(buf);
	fclose(f);
	const char* dot = strrchr(name, '.');
	char base[512]; char ext[64];
	if (dot && dot != name) {
		size_t bl = (size_t)(dot - name);
		if (bl >= sizeof(base)) bl = sizeof(base) - 1;
		memcpy(base, name, bl); base[bl] = 0;
		snprintf(ext, sizeof(ext), "%s", dot);
	} else {
		snprintf(base, sizeof(base), "%s", name);
		ext[0] = 0;
	}
	for (int i = 1; i < 1000; i++) {
		snprintf(buf, sizeof(buf), "%s/%s-%d%s", outDir, base, i, ext);
		f = fopen(buf, "rb");
		if (!f) return strdup(buf);
		fclose(f);
	}
	return strdup(buf);
}

// wt_persist_uri opens an InputStream from resolver.openInputStream(uri) and streams
// it into outDir/<displayName>, returning the resolved path (malloc'd) or NULL.
static char* wt_persist_uri(JNIEnv* env, jobject resolver, jobject uri, const char* outDir) {
	jclass cResolver = (*env)->GetObjectClass(env, resolver);
	jmethodID mOpen = (*env)->GetMethodID(env, cResolver, "openInputStream",
		"(Landroid/net/Uri;)Ljava/io/InputStream;");
	jobject in = (*env)->CallObjectMethod(env, resolver, mOpen, uri);
	(*env)->DeleteLocalRef(env, cResolver);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return NULL; }
	if (!in) return NULL;

	char* name = wt_query_display_name(env, resolver, uri);
	if (!name) {
		// Fallback: derive from Uri.getLastPathSegment().
		jclass cUri = (*env)->GetObjectClass(env, uri);
		jmethodID mLast = (*env)->GetMethodID(env, cUri, "getLastPathSegment", "()Ljava/lang/String;");
		jstring js = (jstring)(*env)->CallObjectMethod(env, uri, mLast);
		if (js) { name = wt_jstring_to_c(env, js); (*env)->DeleteLocalRef(env, js); }
		(*env)->DeleteLocalRef(env, cUri);
	}
	if (!name || name[0] == 0) {
		if (name) free(name);
		name = strdup("shared-audio");
	}
	// Strip any path separators in name to keep us inside outDir.
	for (char* p = name; *p; p++) { if (*p == '/' || *p == '\\') *p = '_'; }

	char* dst = wt_unique_path(outDir, name);
	free(name);
	if (!dst) {
		jclass cIn = (*env)->GetObjectClass(env, in);
		jmethodID mClose = (*env)->GetMethodID(env, cIn, "close", "()V");
		(*env)->CallVoidMethod(env, in, mClose);
		(*env)->DeleteLocalRef(env, cIn);
		(*env)->DeleteLocalRef(env, in);
		return NULL;
	}

	FILE* f = fopen(dst, "wb");
	if (!f) {
		WT_LOGE("fopen failed for %s", dst);
		jclass cIn = (*env)->GetObjectClass(env, in);
		jmethodID mClose = (*env)->GetMethodID(env, cIn, "close", "()V");
		(*env)->CallVoidMethod(env, in, mClose);
		(*env)->DeleteLocalRef(env, cIn);
		(*env)->DeleteLocalRef(env, in);
		free(dst);
		return NULL;
	}

	jclass cIn = (*env)->GetObjectClass(env, in);
	jmethodID mRead  = (*env)->GetMethodID(env, cIn, "read", "([B)I");
	jmethodID mClose = (*env)->GetMethodID(env, cIn, "close", "()V");
	jbyteArray buf = (*env)->NewByteArray(env, 64 * 1024);

	int ok = 1;
	for (;;) {
		jint n = (*env)->CallIntMethod(env, in, mRead, buf);
		if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); ok = 0; break; }
		if (n <= 0) break;
		jbyte* bytes = (*env)->GetByteArrayElements(env, buf, NULL);
		size_t w = fwrite(bytes, 1, (size_t)n, f);
		(*env)->ReleaseByteArrayElements(env, buf, bytes, JNI_ABORT);
		if ((int)w != n) { ok = 0; break; }
	}

	(*env)->DeleteLocalRef(env, buf);
	(*env)->CallVoidMethod(env, in, mClose);
	(*env)->DeleteLocalRef(env, cIn);
	(*env)->DeleteLocalRef(env, in);
	fclose(f);

	if (!ok) {
		remove(dst);
		free(dst);
		return NULL;
	}
	return dst;
}

// wt_intake_intent inspects activity.getIntent() and persists any audio share/view URIs
// into outDir. Caller frees out->paths[i] and out->paths.
//
// Sets a sentinel action ("wt.intake.consumed") on the intent so re-entry on resume does
// not duplicate-import the same payload.
static void wt_intake_intent(JNIEnv* env, jobject activity, const char* outDir, wt_share_result* out) {
	jclass cAct = (*env)->GetObjectClass(env, activity);
	jmethodID mGetIntent = (*env)->GetMethodID(env, cAct, "getIntent", "()Landroid/content/Intent;");
	jobject intent = (*env)->CallObjectMethod(env, activity, mGetIntent);
	(*env)->DeleteLocalRef(env, cAct);
	if (!intent) return;

	jclass cIntent = (*env)->GetObjectClass(env, intent);
	jmethodID mGetAction = (*env)->GetMethodID(env, cIntent, "getAction", "()Ljava/lang/String;");
	jstring jAction = (jstring)(*env)->CallObjectMethod(env, intent, mGetAction);
	char* action = jAction ? wt_jstring_to_c(env, jAction) : NULL;

	int isSend       = action && strcmp(action, "android.intent.action.SEND") == 0;
	int isSendMulti  = action && strcmp(action, "android.intent.action.SEND_MULTIPLE") == 0;
	int isView       = action && strcmp(action, "android.intent.action.VIEW") == 0;

	if (!isSend && !isSendMulti && !isView) {
		if (action) free(action);
		if (jAction) (*env)->DeleteLocalRef(env, jAction);
		(*env)->DeleteLocalRef(env, cIntent);
		(*env)->DeleteLocalRef(env, intent);
		return;
	}

	jmethodID mGetCR = (*env)->GetMethodID(env,
		(*env)->GetObjectClass(env, activity),
		"getContentResolver", "()Landroid/content/ContentResolver;");
	// Fetch via activity again (the cAct above was deleted).
	jclass cAct2 = (*env)->GetObjectClass(env, activity);
	mGetCR = (*env)->GetMethodID(env, cAct2, "getContentResolver", "()Landroid/content/ContentResolver;");
	jobject resolver = (*env)->CallObjectMethod(env, activity, mGetCR);
	(*env)->DeleteLocalRef(env, cAct2);

	if (isView) {
		jmethodID mGetData = (*env)->GetMethodID(env, cIntent, "getData", "()Landroid/net/Uri;");
		jobject uri = (*env)->CallObjectMethod(env, intent, mGetData);
		if (uri) {
			char* p = wt_persist_uri(env, resolver, uri, outDir);
			if (p) { wt_result_push(out, p); free(p); }
			(*env)->DeleteLocalRef(env, uri);
		}
	} else if (isSend) {
		jmethodID mGetParc = (*env)->GetMethodID(env, cIntent, "getParcelableExtra",
			"(Ljava/lang/String;)Landroid/os/Parcelable;");
		jstring extraStream = (*env)->NewStringUTF(env, "android.intent.extra.STREAM");
		jobject uri = (*env)->CallObjectMethod(env, intent, mGetParc, extraStream);
		(*env)->DeleteLocalRef(env, extraStream);
		if (uri) {
			char* p = wt_persist_uri(env, resolver, uri, outDir);
			if (p) { wt_result_push(out, p); free(p); }
			(*env)->DeleteLocalRef(env, uri);
		}
	} else if (isSendMulti) {
		jmethodID mGetParcList = (*env)->GetMethodID(env, cIntent, "getParcelableArrayListExtra",
			"(Ljava/lang/String;)Ljava/util/ArrayList;");
		jstring extraStream = (*env)->NewStringUTF(env, "android.intent.extra.STREAM");
		jobject list = (*env)->CallObjectMethod(env, intent, mGetParcList, extraStream);
		(*env)->DeleteLocalRef(env, extraStream);
		if (list) {
			jclass cList = (*env)->GetObjectClass(env, list);
			jmethodID mSize = (*env)->GetMethodID(env, cList, "size", "()I");
			jmethodID mGet  = (*env)->GetMethodID(env, cList, "get", "(I)Ljava/lang/Object;");
			jint n = (*env)->CallIntMethod(env, list, mSize);
			for (jint i = 0; i < n; i++) {
				jobject uri = (*env)->CallObjectMethod(env, list, mGet, i);
				if (uri) {
					char* p = wt_persist_uri(env, resolver, uri, outDir);
					if (p) { wt_result_push(out, p); free(p); }
					(*env)->DeleteLocalRef(env, uri);
				}
			}
			(*env)->DeleteLocalRef(env, cList);
			(*env)->DeleteLocalRef(env, list);
		}
	}

	// Mark consumed so resume doesn't re-import.
	jmethodID mSetAction = (*env)->GetMethodID(env, cIntent, "setAction",
		"(Ljava/lang/String;)Landroid/content/Intent;");
	jstring consumed = (*env)->NewStringUTF(env, "wt.intake.consumed");
	jobject _ignored = (*env)->CallObjectMethod(env, intent, mSetAction, consumed);
	if (_ignored) (*env)->DeleteLocalRef(env, _ignored);
	(*env)->DeleteLocalRef(env, consumed);

	if (action)  free(action);
	if (jAction) (*env)->DeleteLocalRef(env, jAction);
	(*env)->DeleteLocalRef(env, resolver);
	(*env)->DeleteLocalRef(env, cIntent);
	(*env)->DeleteLocalRef(env, intent);
}

// Entry point invoked from Go via cgo. envPtr/actPtr are uintptr-style handles.
static void wt_run_intake(uintptr_t envPtr, uintptr_t actPtr, const char* outDir, wt_share_result* out) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject activity = (jobject)actPtr;
	wt_intake_intent(env, activity, outDir, out);
}
*/
import "C"

import (
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"fyne.io/fyne/v2/driver"

	shared "github.com/asolopovas/wt/internal"
)

var (
	shareInbox = make(chan string, 32)
	shareOnce  sync.Mutex
)

// shareIntakeChan exposes paths persisted from share/view intents to the GUI layer.
func shareIntakeChan() <-chan string { return shareInbox }

// pollShareIntent inspects the current Activity's Intent and, for SEND/SEND_MULTIPLE/VIEW
// with audio URIs, copies streams into the import cache and pushes paths onto shareInbox.
// Safe to call repeatedly (e.g., on resume); re-entry is a no-op once an intent is consumed.
func pollShareIntent() {
	shareOnce.Lock()
	defer shareOnce.Unlock()

	outDir := filepath.Join(shared.CacheDir(), "imports")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return
	}
	cOutDir := C.CString(outDir)
	defer C.free(unsafe.Pointer(cOutDir))

	var res C.wt_share_result

	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		C.wt_run_intake(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cOutDir, &res)
		return nil
	})

	if res.count == 0 || res.paths == nil {
		return
	}
	n := int(res.count)
	pp := unsafe.Slice(res.paths, n)
	for i := 0; i < n; i++ {
		if pp[i] == nil {
			continue
		}
		path := C.GoString(pp[i])
		C.free(unsafe.Pointer(pp[i]))
		select {
		case shareInbox <- path:
		default:
			// Inbox full — drop oldest by draining one slot, then push.
			select {
			case <-shareInbox:
			default:
			}
			select {
			case shareInbox <- path:
			default:
			}
		}
	}
	C.free(unsafe.Pointer(res.paths))
}
