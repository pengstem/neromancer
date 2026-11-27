package dev.mygo

import android.appwidget.AppWidgetManager
import android.content.ComponentName
import android.content.Context
import android.os.Handler
import android.os.Looper
import org.json.JSONObject

/** Registers the home-screen widget and asks to pin it once the window is actually in front. */
class CaptureWidgetPlugin : MyGoPlugin {
    init {
        askWhenSettled(PluginHost.context, 0)
    }

    override fun call(method: String, args: JSONObject, reply: PluginReply) {
        reply.success(JSONObject())
    }
}

private fun askWhenSettled(context: Context, attempt: Int) {
    Handler(Looper.getMainLooper()).postDelayed({
        try {
            CaptureWidgetProvider.refresh(context)
            val act = PluginHost.activity
            if (act == null || !act.hasWindowFocus()) {
                if (attempt < 12) askWhenSettled(context, attempt + 1)
                return@postDelayed
            }
            maybePin(context)
        } catch (_: Throwable) {
        }
    }, 500)
}

private fun maybePin(context: Context) {
    val mgr = AppWidgetManager.getInstance(context)
    val cn = ComponentName(context, CaptureWidgetProvider::class.java)
    if (mgr.getAppWidgetIds(cn).isNotEmpty()) return
    if (!mgr.isRequestPinAppWidgetSupported) return
    val prefs = context.getSharedPreferences("neromancer_widget", Context.MODE_PRIVATE)
    // A new key so a prompt that was covered by the first-launch sheet can ask once more.
    if (prefs.getBoolean("pin_asked_v2", false)) return
    prefs.edit().putBoolean("pin_asked_v2", true).apply()
    mgr.requestPinAppWidget(cn, null, null)
}
