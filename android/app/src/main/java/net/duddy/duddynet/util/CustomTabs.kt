package net.duddy.duddynet.util

import android.content.ActivityNotFoundException
import android.content.Context
import android.net.Uri
import android.widget.Toast
import androidx.browser.customtabs.CustomTabsIntent

/**
 * Opens a URL in a Chrome Custom Tab (falling back to the system browser). We do
 * NOT build an in-app browser — admin pages open in the real browser, over the
 * already-private Tailscale connection, where the user authenticates directly
 * with the device.
 */
object CustomTabs {
    fun open(context: Context, url: String) {
        val uri = runCatching { Uri.parse(url) }.getOrNull()
        if (uri == null || uri.scheme.isNullOrBlank()) {
            Toast.makeText(context, "Invalid URL", Toast.LENGTH_SHORT).show()
            return
        }
        try {
            CustomTabsIntent.Builder()
                .setShowTitle(true)
                .build()
                .launchUrl(context, uri)
        } catch (_: ActivityNotFoundException) {
            Toast.makeText(context, "No browser available", Toast.LENGTH_SHORT).show()
        }
    }
}
