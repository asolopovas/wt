package com.asolopovas.wtranscribe;

import android.content.ContentProvider;
import android.content.ContentValues;
import android.content.Context;
import android.database.Cursor;
import android.database.MatrixCursor;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import android.provider.OpenableColumns;
import android.webkit.MimeTypeMap;

import java.io.File;
import java.io.FileNotFoundException;

/**
 * Minimal FileProvider for sharing files from the app's cache to other apps
 * via content:// URIs. Only files inside getCacheDir()/share/ may be served.
 *
 * URI form: content://com.asolopovas.wtranscribe.fileprovider/share/<filename>
 */
public class WtFileProvider extends ContentProvider {
    public static final String AUTHORITY = "com.asolopovas.wtranscribe.fileprovider";
    private static final String SHARE_SEGMENT = "share";

    /** Returns the directory under cache where shareable files must live. */
    public static File shareDir(Context ctx) {
        File d = new File(ctx.getCacheDir(), SHARE_SEGMENT);
        if (!d.exists()) d.mkdirs();
        return d;
    }

    /** Build a content:// URI for a file already inside shareDir(). */
    public static Uri uriForName(String name) {
        return new Uri.Builder()
                .scheme("content")
                .authority(AUTHORITY)
                .appendPath(SHARE_SEGMENT)
                .appendPath(name)
                .build();
    }

    @Override
    public boolean onCreate() {
        return true;
    }

    private File resolveFile(Uri uri) throws FileNotFoundException {
        java.util.List<String> seg = uri.getPathSegments();
        if (seg == null || seg.size() < 2 || !SHARE_SEGMENT.equals(seg.get(0))) {
            throw new FileNotFoundException("invalid uri: " + uri);
        }
        Context ctx = getContext();
        if (ctx == null) throw new FileNotFoundException("no context");
        File base = shareDir(ctx);
        File f = new File(base, seg.get(1));
        try {
            String basePath = base.getCanonicalPath();
            String fPath = f.getCanonicalPath();
            if (!fPath.startsWith(basePath + File.separator) && !fPath.equals(basePath)) {
                throw new FileNotFoundException("path escapes share dir");
            }
        } catch (java.io.IOException e) {
            throw new FileNotFoundException(e.getMessage());
        }
        if (!f.exists()) throw new FileNotFoundException(f.getAbsolutePath());
        return f;
    }

    @Override
    public ParcelFileDescriptor openFile(Uri uri, String mode) throws FileNotFoundException {
        File f = resolveFile(uri);
        int m = ParcelFileDescriptor.MODE_READ_ONLY;
        if (mode != null && mode.contains("w")) {
            m = ParcelFileDescriptor.MODE_READ_WRITE;
        }
        return ParcelFileDescriptor.open(f, m);
    }

    @Override
    public String getType(Uri uri) {
        String name = uri.getLastPathSegment();
        if (name == null) return "application/octet-stream";
        int dot = name.lastIndexOf('.');
        if (dot < 0) return "application/octet-stream";
        String ext = name.substring(dot + 1).toLowerCase();
        String mt = MimeTypeMap.getSingleton().getMimeTypeFromExtension(ext);
        if (mt != null) return mt;
        // Fallbacks for office types not always in MimeTypeMap.
        switch (ext) {
            case "txt": return "text/plain";
            case "csv": return "text/csv";
            case "json": return "application/json";
            case "xlsx": return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";
            case "zip": return "application/zip";
            default: return "application/octet-stream";
        }
    }

    @Override
    public Cursor query(Uri uri, String[] projection, String selection,
                        String[] selectionArgs, String sortOrder) {
        File f;
        try {
            f = resolveFile(uri);
        } catch (FileNotFoundException e) {
            return null;
        }
        String[] cols = projection != null
                ? projection
                : new String[]{OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE};
        Object[] row = new Object[cols.length];
        for (int i = 0; i < cols.length; i++) {
            if (OpenableColumns.DISPLAY_NAME.equals(cols[i])) row[i] = f.getName();
            else if (OpenableColumns.SIZE.equals(cols[i])) row[i] = f.length();
            else row[i] = null;
        }
        MatrixCursor mc = new MatrixCursor(cols, 1);
        mc.addRow(row);
        return mc;
    }

    @Override
    public Uri insert(Uri uri, ContentValues values) {
        return null;
    }

    @Override
    public int delete(Uri uri, String selection, String[] selectionArgs) {
        return 0;
    }

    @Override
    public int update(Uri uri, ContentValues values, String selection, String[] selectionArgs) {
        return 0;
    }
}
