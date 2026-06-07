package net.duddy.duddynet.util

import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/** Helpers for formatting the agent's RFC3339 timestamp strings (minSdk 26+). */
object Time {
    private val formatter: DateTimeFormatter =
        DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm").withZone(ZoneId.systemDefault())

    /** Formats an RFC3339 string for display; returns the input on parse failure. */
    fun format(iso: String?): String {
        if (iso.isNullOrBlank()) return "—"
        return runCatching { formatter.format(Instant.parse(iso)) }.getOrDefault(iso)
    }

    /** Short relative-ish label like "just now", "5m ago", or a date. */
    fun relative(iso: String?): String {
        if (iso.isNullOrBlank()) return "never"
        val instant = runCatching { Instant.parse(iso) }.getOrNull() ?: return iso
        val secs = (Instant.now().epochSecond - instant.epochSecond).coerceAtLeast(0)
        return when {
            secs < 60 -> "just now"
            secs < 3600 -> "${secs / 60}m ago"
            secs < 86400 -> "${secs / 3600}h ago"
            else -> format(iso)
        }
    }

    fun relativeMs(epochMs: Long): String = relative(Instant.ofEpochMilli(epochMs).toString())
}
