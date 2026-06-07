package net.duddy.duddynet.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// Brand palette: a calm "private network" green/teal on near-black.
private val Teal = Color(0xFF13A97B)
private val TealDark = Color(0xFF0B3D2E)
private val Mint = Color(0xFFE7FFF5)

internal val StatusGreen = Color(0xFF2ECC71)
internal val StatusRed = Color(0xFFE74C3C)
internal val StatusAmber = Color(0xFFF1C40F)
internal val StatusGray = Color(0xFF95A5A6)

private val DarkColors = darkColorScheme(
    primary = Teal,
    onPrimary = Color.Black,
    primaryContainer = TealDark,
    onPrimaryContainer = Mint,
    secondary = Teal,
    background = Color(0xFF101314),
    surface = Color(0xFF181C1E),
    surfaceVariant = Color(0xFF24292B),
    onBackground = Color(0xFFE6EAEB),
    onSurface = Color(0xFFE6EAEB),
)

private val LightColors = lightColorScheme(
    primary = TealDark,
    onPrimary = Color.White,
    primaryContainer = Mint,
    onPrimaryContainer = TealDark,
    secondary = Teal,
    background = Color(0xFFF7FAF9),
    surface = Color(0xFFFFFFFF),
)

@Composable
fun DuddyNetTheme(
    darkOverride: Boolean? = null,
    content: @Composable () -> Unit,
) {
    val dark = darkOverride ?: isSystemInDarkTheme()
    MaterialTheme(
        colorScheme = if (dark) DarkColors else LightColors,
        typography = Typography(),
        content = content,
    )
}
