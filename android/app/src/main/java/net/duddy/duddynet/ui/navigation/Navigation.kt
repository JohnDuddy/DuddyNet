package net.duddy.duddynet.ui.navigation

import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ListAlt
import androidx.compose.material.icons.filled.Dashboard
import androidx.compose.material.icons.filled.Devices
import androidx.compose.material.icons.filled.PowerSettingsNew
import androidx.compose.material.icons.filled.Settings
import androidx.compose.ui.graphics.vector.ImageVector

/** Route constants and the bottom-navigation definition. */
object Routes {
    const val ONBOARDING = "onboarding"
    const val DASHBOARD = "dashboard"
    const val DEVICES = "devices"
    const val DEVICE_DETAIL = "device/{id}"
    const val SERVICES = "services"
    const val WOL = "wol"
    const val FILES = "files"
    const val LOGS = "logs"
    const val SETTINGS = "settings"

    fun deviceDetail(id: String) = "device/$id"
}

data class BottomTab(val route: String, val label: String, val icon: ImageVector)

val bottomTabs = listOf(
    BottomTab(Routes.DASHBOARD, "Home", Icons.Filled.Dashboard),
    BottomTab(Routes.DEVICES, "Devices", Icons.Filled.Devices),
    BottomTab(Routes.WOL, "Wake", Icons.Filled.PowerSettingsNew),
    BottomTab(Routes.LOGS, "Logs", Icons.AutoMirrored.Filled.ListAlt),
    BottomTab(Routes.SETTINGS, "Settings", Icons.Filled.Settings),
)

/** Routes that show the bottom navigation bar. */
val tabRoutes = bottomTabs.map { it.route }.toSet()
