package com.asolopovas.wtranscribe;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.pm.ApplicationInfo;
import android.content.pm.PackageManager;
import android.os.Build;
import android.os.IBinder;

public class WtForegroundService extends Service {
    private static final String CHANNEL_ID = "wt_transcribe";
    private static final int NOTIF_ID = 1;
    private static volatile WtForegroundService instance;

    @Override
    public void onCreate() {
        super.onCreate();
        instance = this;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel ch = new NotificationChannel(
                    CHANNEL_ID, "Transcription", NotificationManager.IMPORTANCE_LOW);
            ch.setDescription("Keeps wt transcription running with full CPU access");
            NotificationManager nm = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
            if (nm != null) nm.createNotificationChannel(ch);
        }
    }

    private String appLabel() {
        try {
            PackageManager pm = getPackageManager();
            ApplicationInfo ai = pm.getApplicationInfo(getPackageName(), 0);
            CharSequence cs = pm.getApplicationLabel(ai);
            if (cs != null) return cs.toString();
        } catch (Exception ignored) {
        }
        return "wt";
    }

    private Notification buildNotification(int percent, String contentText) {
        Notification.Builder b;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            b = new Notification.Builder(this, CHANNEL_ID);
        } else {
            b = new Notification.Builder(this);
        }
        b.setContentTitle(appLabel() + ": Transcribing")
                .setContentText(contentText)
                .setSmallIcon(android.R.drawable.ic_media_play)
                .setOngoing(true)
                .setOnlyAlertOnce(true);
        if (percent >= 0 && percent <= 100) {
            b.setProgress(100, percent, false);
        } else {
            b.setProgress(0, 0, true);
        }
        return b.build();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        // 2-arg startForeground: targetSdk<=33 doesn't require manifest
        // foregroundServiceType (which gomobile's API-16 resource table cannot represent).
        startForeground(NOTIF_ID, buildNotification(-1, "Starting…"));
        return START_NOT_STICKY;
    }

    @Override
    public void onDestroy() {
        instance = null;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
            stopForeground(Service.STOP_FOREGROUND_REMOVE);
        } else {
            stopForeground(true);
        }
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }

    // Called from native (JNI). Updates the foreground notification in-place.
    // percent: 0..100 for determinate bar, -1 for indeterminate.
    public static void updateProgress(int percent, String contentText) {
        WtForegroundService s = instance;
        if (s == null) return;
        NotificationManager nm = (NotificationManager) s.getSystemService(Context.NOTIFICATION_SERVICE);
        if (nm == null) return;
        nm.notify(NOTIF_ID, s.buildNotification(percent, contentText));
    }
}
