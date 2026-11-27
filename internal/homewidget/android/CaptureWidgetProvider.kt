package dev.mygo

import android.app.PendingIntent
import android.appwidget.AppWidgetManager
import android.appwidget.AppWidgetProvider
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.widget.RemoteViews
import androidx.annotation.Keep

@Keep
class CaptureWidgetProvider : AppWidgetProvider() {
    override fun onUpdate(context: Context, appWidgetManager: AppWidgetManager, appWidgetIds: IntArray) {
        val views = render(context)
        for (id in appWidgetIds) {
            appWidgetManager.updateAppWidget(id, views)
        }
    }

    companion object {
        fun refresh(context: Context) {
            val mgr = AppWidgetManager.getInstance(context)
            val cn = ComponentName(context, CaptureWidgetProvider::class.java)
            val ids = mgr.getAppWidgetIds(cn)
            if (ids.isEmpty()) return
            val views = render(context)
            mgr.updateAppWidget(ids, views)
        }

        private fun render(context: Context): RemoteViews {
            val views = RemoteViews(context.packageName, R.layout.neromancer_widget)
            bind(context, views, R.id.widget_title, "neromancer://capture/text", 1)
            bind(context, views, R.id.widget_say, "neromancer://capture/voice", 2)
            bind(context, views, R.id.widget_write, "neromancer://capture/text", 3)
            bind(context, views, R.id.widget_snap, "neromancer://capture/image", 4)
            return views
        }

        private fun bind(context: Context, views: RemoteViews, viewId: Int, url: String, request: Int) {
            val intent = Intent(Intent.ACTION_VIEW, Uri.parse(url)).apply {
                setPackage(context.packageName)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP)
            }
            val flags = PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
            val pending = PendingIntent.getActivity(context, request, intent, flags)
            views.setOnClickPendingIntent(viewId, pending)
        }
    }
}
