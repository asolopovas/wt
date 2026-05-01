package com.asolopovas.wtranscribe;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import android.os.IBinder;

public class WtForegroundService extends Service {
    private static final String CHANNEL_ID = "wt_transcribe";
    private static final int NOTIF_ID = 1;

    @Override
    public void onCreate() {
        super.onCreate();
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel ch = new NotificationChannel(
                    CHANNEL_ID, "Transcription", NotificationManager.IMPORTANCE_LOW);
            ch.setDescription("Keeps wt transcription running with full CPU access");
            NotificationManager nm = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
            if (nm != null) nm.createNotificationChannel(ch);
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        Notification.Builder b;
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            b = new Notification.Builder(this, CHANNEL_ID);
        } else {
            b = new Notification.Builder(this);
        }
        b.setContentTitle("wt")
                .setContentText("Transcribing audio")
                .setSmallIcon(android.R.drawable.ic_media_play)
                .setOngoing(true);
        Notification n = b.build();
        // Don't pass a foregroundServiceType at runtime: gomobile pins the
        // resource table to API 16, so we cannot declare the matching
        // foregroundServiceType attr in the manifest. With targetSdkVersion
        // <= 33, the 2-arg startForeground is permitted without a type.
        startForeground(NOTIF_ID, n);
        return START_NOT_STICKY;
    }

    @Override
    public void onDestroy() {
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
}
