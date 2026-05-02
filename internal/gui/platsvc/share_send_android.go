//go:build android

package platsvc

/*
#cgo LDFLAGS: -llog

#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include <android/log.h>

#define WTS_TAG "wtshare"
#define WTS_LOGE(...) __android_log_print(ANDROID_LOG_ERROR, WTS_TAG, __VA_ARGS__)
#define WTS_LOGI(...) __android_log_print(ANDROID_LOG_INFO,  WTS_TAG, __VA_ARGS__)

// Load an application class via the activity's ClassLoader (android.app.Activity
// is loaded by the boot loader and FindClass cannot resolve our classes from
// arbitrary threads).
static jclass wts_load_app_class(JNIEnv* env, jobject ctx, const char* dotted) {
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

static jobject wts_new_intent(JNIEnv* env, const char* action) {
	jclass cIntent = (*env)->FindClass(env, "android/content/Intent");
	jmethodID mInit = (*env)->GetMethodID(env, cIntent, "<init>", "(Ljava/lang/String;)V");
	jstring jA = (*env)->NewStringUTF(env, action);
	jobject intent = (*env)->NewObject(env, cIntent, mInit, jA);
	(*env)->DeleteLocalRef(env, jA);
	(*env)->DeleteLocalRef(env, cIntent);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return NULL; }
	return intent;
}

static void wts_intent_set_type(JNIEnv* env, jobject intent, const char* mime) {
	jclass cIntent = (*env)->GetObjectClass(env, intent);
	jmethodID mSetType = (*env)->GetMethodID(env, cIntent, "setType", "(Ljava/lang/String;)Landroid/content/Intent;");
	jstring jM = (*env)->NewStringUTF(env, mime);
	jobject r = (*env)->CallObjectMethod(env, intent, mSetType, jM);
	if (r) (*env)->DeleteLocalRef(env, r);
	(*env)->DeleteLocalRef(env, jM);
	(*env)->DeleteLocalRef(env, cIntent);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
}

static void wts_intent_put_string_extra(JNIEnv* env, jobject intent, const char* key, const char* value) {
	jclass cIntent = (*env)->GetObjectClass(env, intent);
	jmethodID mPut = (*env)->GetMethodID(env, cIntent, "putExtra",
		"(Ljava/lang/String;Ljava/lang/String;)Landroid/content/Intent;");
	jstring jK = (*env)->NewStringUTF(env, key);
	jstring jV = (*env)->NewStringUTF(env, value);
	jobject r = (*env)->CallObjectMethod(env, intent, mPut, jK, jV);
	if (r) (*env)->DeleteLocalRef(env, r);
	(*env)->DeleteLocalRef(env, jK);
	(*env)->DeleteLocalRef(env, jV);
	(*env)->DeleteLocalRef(env, cIntent);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
}

static void wts_intent_put_parcelable_extra(JNIEnv* env, jobject intent, const char* key, jobject value) {
	jclass cIntent = (*env)->GetObjectClass(env, intent);
	jmethodID mPut = (*env)->GetMethodID(env, cIntent, "putExtra",
		"(Ljava/lang/String;Landroid/os/Parcelable;)Landroid/content/Intent;");
	jstring jK = (*env)->NewStringUTF(env, key);
	jobject r = (*env)->CallObjectMethod(env, intent, mPut, jK, value);
	if (r) (*env)->DeleteLocalRef(env, r);
	(*env)->DeleteLocalRef(env, jK);
	(*env)->DeleteLocalRef(env, cIntent);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
}

static void wts_intent_add_flags(JNIEnv* env, jobject intent, jint flags) {
	jclass cIntent = (*env)->GetObjectClass(env, intent);
	jmethodID mAdd = (*env)->GetMethodID(env, cIntent, "addFlags", "(I)Landroid/content/Intent;");
	jobject r = (*env)->CallObjectMethod(env, intent, mAdd, flags);
	if (r) (*env)->DeleteLocalRef(env, r);
	(*env)->DeleteLocalRef(env, cIntent);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
}

static jobject wts_make_chooser(JNIEnv* env, jobject inner, const char* title) {
	jclass cIntent = (*env)->FindClass(env, "android/content/Intent");
	jmethodID mChoose = (*env)->GetStaticMethodID(env, cIntent, "createChooser",
		"(Landroid/content/Intent;Ljava/lang/CharSequence;)Landroid/content/Intent;");
	jstring jT = (*env)->NewStringUTF(env, title ? title : "Share");
	jobject chooser = (*env)->CallStaticObjectMethod(env, cIntent, mChoose, inner, jT);
	(*env)->DeleteLocalRef(env, jT);
	(*env)->DeleteLocalRef(env, cIntent);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return NULL; }
	return chooser;
}

static int wts_start_activity(JNIEnv* env, jobject ctx, jobject intent) {
	wts_intent_add_flags(env, intent, 0x10000000); // FLAG_ACTIVITY_NEW_TASK
	jclass cCtx = (*env)->GetObjectClass(env, ctx);
	jmethodID mStart = (*env)->GetMethodID(env, cCtx, "startActivity", "(Landroid/content/Intent;)V");
	(*env)->CallVoidMethod(env, ctx, mStart, intent);
	(*env)->DeleteLocalRef(env, cCtx);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionDescribe(env);
		(*env)->ExceptionClear(env);
		return -1;
	}
	return 0;
}

static int wts_share_text(uintptr_t envPtr, uintptr_t ctxPtr, const char* text, const char* subject) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject ctx = (jobject)ctxPtr;
	if (!env || !ctx || !text) return -1;

	jobject intent = wts_new_intent(env, "android.intent.action.SEND");
	if (!intent) return -1;

	wts_intent_set_type(env, intent, "text/plain");
	wts_intent_put_string_extra(env, intent, "android.intent.extra.TEXT", text);
	if (subject && subject[0]) {
		wts_intent_put_string_extra(env, intent, "android.intent.extra.SUBJECT", subject);
	}

	jobject chooser = wts_make_chooser(env, intent, "Share transcript");
	(*env)->DeleteLocalRef(env, intent);
	if (!chooser) return -1;

	int rc = wts_start_activity(env, ctx, chooser);
	(*env)->DeleteLocalRef(env, chooser);
	return rc;
}

// Build a content:// URI for a file inside the FileProvider's share dir by
// invoking the static helper WtFileProvider.uriForName(String).
static jobject wts_provider_uri_for_name(JNIEnv* env, jobject ctx, const char* name) {
	jclass cFP = wts_load_app_class(env, ctx, "com.asolopovas.wtranscribe.WtFileProvider");
	if (!cFP) { WTS_LOGE("WtFileProvider class not loadable"); return NULL; }
	jmethodID mFor = (*env)->GetStaticMethodID(env, cFP, "uriForName",
		"(Ljava/lang/String;)Landroid/net/Uri;");
	if (!mFor) {
		(*env)->ExceptionClear(env);
		(*env)->DeleteLocalRef(env, cFP);
		WTS_LOGE("WtFileProvider.uriForName missing");
		return NULL;
	}
	jstring jN = (*env)->NewStringUTF(env, name);
	jobject uri = (*env)->CallStaticObjectMethod(env, cFP, mFor, jN);
	(*env)->DeleteLocalRef(env, jN);
	(*env)->DeleteLocalRef(env, cFP);
	if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); return NULL; }
	return uri;
}

// Returns the absolute path of WtFileProvider.shareDir(ctx). Caller must free.
static char* wts_share_dir(JNIEnv* env, jobject ctx) {
	jclass cFP = wts_load_app_class(env, ctx, "com.asolopovas.wtranscribe.WtFileProvider");
	if (!cFP) return NULL;
	jmethodID mDir = (*env)->GetStaticMethodID(env, cFP, "shareDir",
		"(Landroid/content/Context;)Ljava/io/File;");
	if (!mDir) { (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, cFP); return NULL; }
	jobject file = (*env)->CallStaticObjectMethod(env, cFP, mDir, ctx);
	(*env)->DeleteLocalRef(env, cFP);
	if ((*env)->ExceptionCheck(env) || !file) { (*env)->ExceptionClear(env); return NULL; }

	jclass cFile = (*env)->GetObjectClass(env, file);
	jmethodID mAbs = (*env)->GetMethodID(env, cFile, "getAbsolutePath", "()Ljava/lang/String;");
	jstring jAbs = (jstring)(*env)->CallObjectMethod(env, file, mAbs);
	(*env)->DeleteLocalRef(env, cFile);
	(*env)->DeleteLocalRef(env, file);
	if ((*env)->ExceptionCheck(env) || !jAbs) { (*env)->ExceptionClear(env); return NULL; }

	const char* utf = (*env)->GetStringUTFChars(env, jAbs, NULL);
	char* out = utf ? strdup(utf) : NULL;
	if (utf) (*env)->ReleaseStringUTFChars(env, jAbs, utf);
	(*env)->DeleteLocalRef(env, jAbs);
	return out;
}

static int wts_share_file_uri(uintptr_t envPtr, uintptr_t ctxPtr,
                              const char* providerName, const char* mime, const char* subject) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject ctx = (jobject)ctxPtr;
	if (!env || !ctx || !providerName || !mime) return -1;

	jobject uri = wts_provider_uri_for_name(env, ctx, providerName);
	if (!uri) return -1;

	jobject intent = wts_new_intent(env, "android.intent.action.SEND");
	if (!intent) { (*env)->DeleteLocalRef(env, uri); return -1; }

	wts_intent_set_type(env, intent, mime);
	wts_intent_put_parcelable_extra(env, intent, "android.intent.extra.STREAM", uri);
	if (subject && subject[0]) {
		wts_intent_put_string_extra(env, intent, "android.intent.extra.SUBJECT", subject);
	}
	// FLAG_GRANT_READ_URI_PERMISSION
	wts_intent_add_flags(env, intent, 0x00000001);
	(*env)->DeleteLocalRef(env, uri);

	jobject chooser = wts_make_chooser(env, intent, "Share transcript");
	(*env)->DeleteLocalRef(env, intent);
	if (!chooser) return -1;

	int rc = wts_start_activity(env, ctx, chooser);
	(*env)->DeleteLocalRef(env, chooser);
	return rc;
}

static char* wts_get_share_dir(uintptr_t envPtr, uintptr_t ctxPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject ctx = (jobject)ctxPtr;
	if (!env || !ctx) return NULL;
	return wts_share_dir(env, ctx);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"fyne.io/fyne/v2/driver"
)

// ErrShareUnsupported keeps API parity with the desktop stub.
var ErrShareUnsupported = errors.New("native share unavailable")

// ShareSupported always returns true on Android (the actual call may still
// fail if the activity is not foreground).
func ShareSupported() bool { return true }

// ShareText opens the system share sheet with plain-text content. WhatsApp
// and most messengers render this directly in the message body.
func ShareText(text, subject string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("share: empty text")
	}
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	cSubj := C.CString(subject)
	defer C.free(unsafe.Pointer(cSubj))

	var rc C.int = -1
	err := driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return errors.New("share: no Android context")
		}
		rc = C.wts_share_text(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cText, cSubj)
		return nil
	})
	if err != nil {
		return err
	}
	if rc != 0 {
		return errors.New("share: startActivity failed")
	}
	return nil
}

// ShareFile opens the system share sheet for the file at srcPath. The file is
// copied into the FileProvider's cache/share directory and exposed via a
// content:// URI so other apps (WhatsApp, Gmail, Drive, etc.) can read it.
// mime should be the file's MIME type (e.g. "text/csv"); subject is optional.
func ShareFile(srcPath, mime, subject string) error {
	if srcPath == "" {
		return errors.New("share: empty path")
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("share: open source: %w", err)
	}
	defer func() { _ = src.Close() }()

	var shareDir string
	err = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return errors.New("share: no Android context")
		}
		c := C.wts_get_share_dir(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx))
		if c == nil {
			return errors.New("share: cannot resolve provider dir")
		}
		shareDir = C.GoString(c)
		C.free(unsafe.Pointer(c))
		return nil
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(shareDir, 0o755); err != nil {
		return fmt.Errorf("share: mkdir: %w", err)
	}

	name := sanitizeShareName(filepath.Base(srcPath))
	dst := filepath.Join(shareDir, name)
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("share: stage: %w", err)
	}
	if _, err := io.Copy(out, src); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return fmt.Errorf("share: copy: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("share: close: %w", err)
	}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	cMime := C.CString(mime)
	defer C.free(unsafe.Pointer(cMime))
	cSubj := C.CString(subject)
	defer C.free(unsafe.Pointer(cSubj))

	var rc C.int = -1
	err = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return errors.New("share: no Android context")
		}
		rc = C.wts_share_file_uri(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx), cName, cMime, cSubj)
		return nil
	})
	if err != nil {
		return err
	}
	if rc != 0 {
		return errors.New("share: startActivity failed")
	}
	return nil
}

func sanitizeShareName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "share.bin"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == 0:
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
