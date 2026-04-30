//go:build android

package gui

/*
#cgo LDFLAGS: -llog

#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <sys/stat.h>
#include <android/log.h>

#define WR_TAG "wtrec"
#define WR_LOGI(...) __android_log_print(ANDROID_LOG_INFO, WR_TAG, __VA_ARGS__)
#define WR_LOGE(...) __android_log_print(ANDROID_LOG_ERROR, WR_TAG, __VA_ARGS__)

static jobject g_recorder = NULL;

// wt_rec_start: creates MediaRecorder, configures AAC/MP4, writes to outPath, start().
// Returns 1 on success.
static int wt_rec_start(uintptr_t envPtr, const char* outPath) {
	JNIEnv* env = (JNIEnv*)envPtr;
	if (!env || g_recorder) return 0;

	jclass cMR = (*env)->FindClass(env, "android/media/MediaRecorder");
	if (!cMR) { (*env)->ExceptionClear(env); return 0; }
	jmethodID mInit = (*env)->GetMethodID(env, cMR, "<init>", "()V");
	jobject mr = (*env)->NewObject(env, cMR, mInit);
	if (!mr) { (*env)->ExceptionClear(env); (*env)->DeleteLocalRef(env, cMR); return 0; }

	// MediaRecorder.AudioSource.MIC = 1
	jmethodID mSetSrc = (*env)->GetMethodID(env, cMR, "setAudioSource", "(I)V");
	(*env)->CallVoidMethod(env, mr, mSetSrc, (jint)1);
	if ((*env)->ExceptionCheck(env)) goto fail;

	// OutputFormat.MPEG_4 = 2
	jmethodID mSetFmt = (*env)->GetMethodID(env, cMR, "setOutputFormat", "(I)V");
	(*env)->CallVoidMethod(env, mr, mSetFmt, (jint)2);
	if ((*env)->ExceptionCheck(env)) goto fail;

	// AudioEncoder.AAC = 3
	jmethodID mSetEnc = (*env)->GetMethodID(env, cMR, "setAudioEncoder", "(I)V");
	(*env)->CallVoidMethod(env, mr, mSetEnc, (jint)3);
	if ((*env)->ExceptionCheck(env)) goto fail;

	jmethodID mSetCh = (*env)->GetMethodID(env, cMR, "setAudioChannels", "(I)V");
	if (mSetCh) (*env)->CallVoidMethod(env, mr, mSetCh, (jint)1);
	jmethodID mSetSR = (*env)->GetMethodID(env, cMR, "setAudioSamplingRate", "(I)V");
	if (mSetSR) (*env)->CallVoidMethod(env, mr, mSetSR, (jint)16000);
	jmethodID mSetBR = (*env)->GetMethodID(env, cMR, "setAudioEncodingBitRate", "(I)V");
	if (mSetBR) (*env)->CallVoidMethod(env, mr, mSetBR, (jint)64000);

	jmethodID mSetFile = (*env)->GetMethodID(env, cMR, "setOutputFile", "(Ljava/lang/String;)V");
	jstring jPath = (*env)->NewStringUTF(env, outPath);
	(*env)->CallVoidMethod(env, mr, mSetFile, jPath);
	(*env)->DeleteLocalRef(env, jPath);
	if ((*env)->ExceptionCheck(env)) goto fail;

	jmethodID mPrep = (*env)->GetMethodID(env, cMR, "prepare", "()V");
	(*env)->CallVoidMethod(env, mr, mPrep);
	if ((*env)->ExceptionCheck(env)) goto fail;

	jmethodID mStart = (*env)->GetMethodID(env, cMR, "start", "()V");
	(*env)->CallVoidMethod(env, mr, mStart);
	if ((*env)->ExceptionCheck(env)) goto fail;

	g_recorder = (*env)->NewGlobalRef(env, mr);
	(*env)->DeleteLocalRef(env, mr);
	(*env)->DeleteLocalRef(env, cMR);
	WR_LOGI("recording started: %s", outPath);
	return 1;

fail:
	(*env)->ExceptionClear(env);
	jmethodID mRel = (*env)->GetMethodID(env, cMR, "release", "()V");
	if (mRel) (*env)->CallVoidMethod(env, mr, mRel);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
	(*env)->DeleteLocalRef(env, mr);
	(*env)->DeleteLocalRef(env, cMR);
	WR_LOGE("recording start failed");
	return 0;
}

static int wt_rec_stop(uintptr_t envPtr) {
	JNIEnv* env = (JNIEnv*)envPtr;
	if (!env || !g_recorder) return 0;

	jclass cMR = (*env)->GetObjectClass(env, g_recorder);
	jmethodID mStop = (*env)->GetMethodID(env, cMR, "stop", "()V");
	(*env)->CallVoidMethod(env, g_recorder, mStop);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);

	jmethodID mReset = (*env)->GetMethodID(env, cMR, "reset", "()V");
	if (mReset) (*env)->CallVoidMethod(env, g_recorder, mReset);
	jmethodID mRel = (*env)->GetMethodID(env, cMR, "release", "()V");
	if (mRel) (*env)->CallVoidMethod(env, g_recorder, mRel);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);

	(*env)->DeleteGlobalRef(env, g_recorder);
	g_recorder = NULL;
	(*env)->DeleteLocalRef(env, cMR);
	WR_LOGI("recording stopped");
	return 1;
}

// wt_publish_to_documents copies the file at srcPath to MediaStore Documents/wt/<displayName>.
// Returns 1 on success. On API 28- it writes to Environment.DIRECTORY_DOCUMENTS/wt directly.
static int wt_publish_to_documents(uintptr_t envPtr, uintptr_t actPtr,
		const char* srcPath, const char* displayName, const char* mimeType, int sdkInt) {
	JNIEnv* env = (JNIEnv*)envPtr;
	jobject act = (jobject)actPtr;
	if (!env || !act) return 0;

	if (sdkInt >= 29) {
		jclass cValues = (*env)->FindClass(env, "android/content/ContentValues");
		jmethodID mInitV = (*env)->GetMethodID(env, cValues, "<init>", "()V");
		jobject vals = (*env)->NewObject(env, cValues, mInitV);
		jmethodID mPutS = (*env)->GetMethodID(env, cValues, "put",
			"(Ljava/lang/String;Ljava/lang/String;)V");
		jmethodID mPutI = (*env)->GetMethodID(env, cValues, "put",
			"(Ljava/lang/String;Ljava/lang/Integer;)V");

		jstring kName = (*env)->NewStringUTF(env, "_display_name");
		jstring vName = (*env)->NewStringUTF(env, displayName);
		(*env)->CallVoidMethod(env, vals, mPutS, kName, vName);
		(*env)->DeleteLocalRef(env, kName); (*env)->DeleteLocalRef(env, vName);

		jstring kMime = (*env)->NewStringUTF(env, "mime_type");
		jstring vMime = (*env)->NewStringUTF(env, mimeType);
		(*env)->CallVoidMethod(env, vals, mPutS, kMime, vMime);
		(*env)->DeleteLocalRef(env, kMime); (*env)->DeleteLocalRef(env, vMime);

		jstring kPath = (*env)->NewStringUTF(env, "relative_path");
		jstring vPath = (*env)->NewStringUTF(env, "Documents/wt");
		(*env)->CallVoidMethod(env, vals, mPutS, kPath, vPath);
		(*env)->DeleteLocalRef(env, kPath); (*env)->DeleteLocalRef(env, vPath);

		jclass cInteger = (*env)->FindClass(env, "java/lang/Integer");
		jmethodID mIntInit = (*env)->GetMethodID(env, cInteger, "<init>", "(I)V");
		jobject pendingOne = (*env)->NewObject(env, cInteger, mIntInit, (jint)1);
		jstring kPending = (*env)->NewStringUTF(env, "is_pending");
		(*env)->CallVoidMethod(env, vals, mPutI, kPending, pendingOne);
		(*env)->DeleteLocalRef(env, kPending); (*env)->DeleteLocalRef(env, pendingOne);
		(*env)->DeleteLocalRef(env, cInteger);

		// Resolver
		jclass cAct = (*env)->GetObjectClass(env, act);
		jmethodID mGetCR = (*env)->GetMethodID(env, cAct, "getContentResolver",
			"()Landroid/content/ContentResolver;");
		jobject resolver = (*env)->CallObjectMethod(env, act, mGetCR);
		(*env)->DeleteLocalRef(env, cAct);

		// Files.getContentUri("external")
		jclass cFiles = (*env)->FindClass(env, "android/provider/MediaStore$Files");
		jmethodID mGetURI = (*env)->GetStaticMethodID(env, cFiles, "getContentUri",
			"(Ljava/lang/String;)Landroid/net/Uri;");
		jstring jExt = (*env)->NewStringUTF(env, "external");
		jobject baseUri = (*env)->CallStaticObjectMethod(env, cFiles, mGetURI, jExt);
		(*env)->DeleteLocalRef(env, jExt);
		(*env)->DeleteLocalRef(env, cFiles);

		jclass cResolver = (*env)->GetObjectClass(env, resolver);
		jmethodID mInsert = (*env)->GetMethodID(env, cResolver, "insert",
			"(Landroid/net/Uri;Landroid/content/ContentValues;)Landroid/net/Uri;");
		jobject newUri = (*env)->CallObjectMethod(env, resolver, mInsert, baseUri, vals);
		(*env)->DeleteLocalRef(env, baseUri);
		(*env)->DeleteLocalRef(env, vals);
		(*env)->DeleteLocalRef(env, cValues);
		if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); newUri = NULL; }
		if (!newUri) { (*env)->DeleteLocalRef(env, cResolver); (*env)->DeleteLocalRef(env, resolver); return 0; }

		jmethodID mOpenOut = (*env)->GetMethodID(env, cResolver, "openOutputStream",
			"(Landroid/net/Uri;)Ljava/io/OutputStream;");
		jobject out = (*env)->CallObjectMethod(env, resolver, mOpenOut, newUri);
		if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); out = NULL; }

		int ok = 0;
		if (out) {
			FILE* f = fopen(srcPath, "rb");
			if (f) {
				jclass cOut = (*env)->GetObjectClass(env, out);
				jmethodID mWrite = (*env)->GetMethodID(env, cOut, "write", "([B)V");
				jmethodID mClose = (*env)->GetMethodID(env, cOut, "close", "()V");
				jbyteArray buf = (*env)->NewByteArray(env, 64 * 1024);
				char cbuf[64 * 1024];
				ok = 1;
				for (;;) {
					size_t n = fread(cbuf, 1, sizeof(cbuf), f);
					if (n == 0) break;
					(*env)->SetByteArrayRegion(env, buf, 0, (jsize)n, (jbyte*)cbuf);
					if (n < sizeof(cbuf)) {
						jbyteArray small = (*env)->NewByteArray(env, (jsize)n);
						(*env)->SetByteArrayRegion(env, small, 0, (jsize)n, (jbyte*)cbuf);
						(*env)->CallVoidMethod(env, out, mWrite, small);
						(*env)->DeleteLocalRef(env, small);
					} else {
						(*env)->CallVoidMethod(env, out, mWrite, buf);
					}
					if ((*env)->ExceptionCheck(env)) { (*env)->ExceptionClear(env); ok = 0; break; }
				}
				(*env)->DeleteLocalRef(env, buf);
				(*env)->CallVoidMethod(env, out, mClose);
				if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
				(*env)->DeleteLocalRef(env, cOut);
				fclose(f);
			}
			(*env)->DeleteLocalRef(env, out);
		}

		// Clear is_pending=0
		jclass cValues2 = (*env)->FindClass(env, "android/content/ContentValues");
		jobject vals2 = (*env)->NewObject(env, cValues2, mInitV);
		jmethodID mPutI2 = (*env)->GetMethodID(env, cValues2, "put",
			"(Ljava/lang/String;Ljava/lang/Integer;)V");
		jclass cInteger2 = (*env)->FindClass(env, "java/lang/Integer");
		jmethodID mIntInit2 = (*env)->GetMethodID(env, cInteger2, "<init>", "(I)V");
		jobject pendingZero = (*env)->NewObject(env, cInteger2, mIntInit2, (jint)0);
		jstring kPending2 = (*env)->NewStringUTF(env, "is_pending");
		(*env)->CallVoidMethod(env, vals2, mPutI2, kPending2, pendingZero);
		(*env)->DeleteLocalRef(env, kPending2);
		(*env)->DeleteLocalRef(env, pendingZero);
		(*env)->DeleteLocalRef(env, cInteger2);
		jmethodID mUpd = (*env)->GetMethodID(env, cResolver, "update",
			"(Landroid/net/Uri;Landroid/content/ContentValues;Ljava/lang/String;[Ljava/lang/String;)I");
		(*env)->CallIntMethod(env, resolver, mUpd, newUri, vals2, NULL, NULL);
		if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
		(*env)->DeleteLocalRef(env, vals2);
		(*env)->DeleteLocalRef(env, cValues2);

		(*env)->DeleteLocalRef(env, newUri);
		(*env)->DeleteLocalRef(env, cResolver);
		(*env)->DeleteLocalRef(env, resolver);
		return ok;
	}

	// Pre-Q: write directly to /sdcard/Documents/wt/<displayName>.
	jclass cEnv = (*env)->FindClass(env, "android/os/Environment");
	jmethodID mGetPub = (*env)->GetStaticMethodID(env, cEnv, "getExternalStoragePublicDirectory",
		"(Ljava/lang/String;)Ljava/io/File;");
	jstring jDocs = (*env)->NewStringUTF(env, "Documents");
	jobject dirFile = (*env)->CallStaticObjectMethod(env, cEnv, mGetPub, jDocs);
	(*env)->DeleteLocalRef(env, jDocs);
	(*env)->DeleteLocalRef(env, cEnv);
	if (!dirFile) return 0;

	jclass cFile = (*env)->GetObjectClass(env, dirFile);
	jmethodID mGetPath = (*env)->GetMethodID(env, cFile, "getAbsolutePath", "()Ljava/lang/String;");
	jstring jPath = (jstring)(*env)->CallObjectMethod(env, dirFile, mGetPath);
	(*env)->DeleteLocalRef(env, cFile);
	(*env)->DeleteLocalRef(env, dirFile);
	const char* docPath = (*env)->GetStringUTFChars(env, jPath, NULL);
	char dst[1024];
	snprintf(dst, sizeof(dst), "%s/wt", docPath);
	(*env)->ReleaseStringUTFChars(env, jPath, docPath);
	(*env)->DeleteLocalRef(env, jPath);

	mkdir(dst, 0755);
	char dstFile[1024];
	snprintf(dstFile, sizeof(dstFile), "%s/%s", dst, displayName);

	FILE* in = fopen(srcPath, "rb");
	if (!in) return 0;
	FILE* outf = fopen(dstFile, "wb");
	if (!outf) { fclose(in); return 0; }
	char cbuf[64 * 1024];
	int ok = 1;
	for (;;) {
		size_t n = fread(cbuf, 1, sizeof(cbuf), in);
		if (n == 0) break;
		if (fwrite(cbuf, 1, n, outf) != n) { ok = 0; break; }
	}
	fclose(in); fclose(outf);
	return ok;
}
*/
import "C"

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"

	"fyne.io/fyne/v2/driver"

	shared "github.com/asolopovas/wt/internal"
)

var (
	recMu      sync.Mutex
	recCurrent string
)

func recordingsDir() string {
	d := filepath.Join(shared.CacheDir(), "recordings")
	_ = os.MkdirAll(d, 0o755)
	return d
}

func startRecording() (string, error) {
	recMu.Lock()
	defer recMu.Unlock()
	if recCurrent != "" {
		return "", fmt.Errorf("already recording")
	}
	name := fmt.Sprintf("rec_%s.m4a", time.Now().Format("20060102_150405"))
	out := filepath.Join(recordingsDir(), name)
	cOut := C.CString(out)
	defer C.free(unsafe.Pointer(cOut))

	ok := false
	_ = driver.RunNative(func(ctx any) error {
		ac, valid := ctx.(*driver.AndroidContext)
		if !valid || ac == nil || ac.Env == 0 {
			return nil
		}
		ok = C.wt_rec_start(C.uintptr_t(ac.Env), cOut) == 1
		return nil
	})
	if !ok {
		return "", fmt.Errorf("MediaRecorder failed (mic permission?)")
	}
	recCurrent = out
	return out, nil
}

func stopRecording() (string, error) {
	recMu.Lock()
	path := recCurrent
	recMu.Unlock()
	if path == "" {
		return "", fmt.Errorf("not recording")
	}
	_ = driver.RunNative(func(ctx any) error {
		ac, ok := ctx.(*driver.AndroidContext)
		if !ok || ac == nil || ac.Env == 0 {
			return nil
		}
		C.wt_rec_stop(C.uintptr_t(ac.Env))
		return nil
	})
	recMu.Lock()
	recCurrent = ""
	recMu.Unlock()
	return path, nil
}

func publishRecordingToDocuments(srcPath string) error {
	displayName := filepath.Base(srcPath)
	cSrc := C.CString(srcPath)
	cName := C.CString(displayName)
	cMime := C.CString("audio/mp4")
	defer C.free(unsafe.Pointer(cSrc))
	defer C.free(unsafe.Pointer(cName))
	defer C.free(unsafe.Pointer(cMime))

	sdk := androidSDKInt()
	ok := false
	_ = driver.RunNative(func(ctx any) error {
		ac, valid := ctx.(*driver.AndroidContext)
		if !valid || ac == nil || ac.Env == 0 || ac.Ctx == 0 {
			return nil
		}
		ok = C.wt_publish_to_documents(C.uintptr_t(ac.Env), C.uintptr_t(ac.Ctx),
			cSrc, cName, cMime, C.int(sdk)) == 1
		return nil
	})
	if !ok {
		return fmt.Errorf("could not save to Documents/wt")
	}
	return nil
}

func isRecording() bool {
	recMu.Lock()
	defer recMu.Unlock()
	return recCurrent != ""
}
