package net.duddy.duddynet.ui.components

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import net.duddy.duddynet.ui.theme.StatusAmber
import net.duddy.duddynet.ui.theme.StatusGray
import net.duddy.duddynet.ui.theme.StatusGreen
import net.duddy.duddynet.ui.theme.StatusRed

/** Coarse reachability status used across the UI. */
enum class Status(val label: String, val color: Color) {
    ONLINE("Online", StatusGreen),
    OFFLINE("Offline", StatusRed),
    SLOW("Slow", StatusAmber),
    UNKNOWN("Unknown", StatusGray);

    companion object {
        /** Derives a status from a health check result. */
        fun fromHealth(online: Boolean?, latencyMs: Long?): Status = when {
            online == null -> UNKNOWN
            !online -> OFFLINE
            latencyMs != null && latencyMs > 400 -> SLOW
            else -> ONLINE
        }
    }
}

@Composable
fun StatusDot(status: Status, modifier: Modifier = Modifier) {
    Box(
        modifier
            .size(14.dp)
            .clip(CircleShape)
            .then(Modifier),
    ) {
        Surface(color = status.color, shape = CircleShape, modifier = Modifier.fillMaxSize()) {}
    }
}

@Composable
fun StatusBadge(status: Status, modifier: Modifier = Modifier) {
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        StatusDot(status)
        Spacer(Modifier.width(6.dp))
        Text(status.label, style = MaterialTheme.typography.labelLarge, fontWeight = FontWeight.Medium)
    }
}

@Composable
fun SectionCard(title: String? = null, content: @Composable () -> Unit) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 6.dp),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
    ) {
        Column(Modifier.padding(16.dp)) {
            if (title != null) {
                Text(title, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
                Spacer(Modifier.size(8.dp))
            }
            content()
        }
    }
}

@Composable
fun LabeledRow(label: String, value: String) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(label, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(value, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium)
    }
}

@Composable
fun LoadingBox(modifier: Modifier = Modifier) {
    Box(modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        CircularProgressIndicator()
    }
}

@Composable
fun MessageBox(message: String, modifier: Modifier = Modifier) {
    Box(modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
        Text(message, style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
