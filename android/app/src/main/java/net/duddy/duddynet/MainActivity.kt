package net.duddy.duddynet

import android.os.Bundle
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.fragment.app.FragmentActivity
import net.duddy.duddynet.data.SettingsStore
import net.duddy.duddynet.di.ServiceLocator
import net.duddy.duddynet.ui.navigation.DuddyNavRoot
import net.duddy.duddynet.ui.theme.DuddyNetTheme
import net.duddy.duddynet.util.Biometric

class MainActivity : FragmentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        val container = ServiceLocator.init(this)

        setContent {
            val settings by container.settingsStore.settings.collectAsState(initial = SettingsStore.Settings())
            val dark = when (settings.darkMode) {
                SettingsStore.DarkMode.SYSTEM -> null
                SettingsStore.DarkMode.LIGHT -> false
                SettingsStore.DarkMode.DARK -> true
            }
            DuddyNetTheme(darkOverride = dark) {
                Surface(Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
                    if (settings.requireBiometric) {
                        BiometricGate { DuddyNavRoot() }
                    } else {
                        DuddyNavRoot()
                    }
                }
            }
        }
    }
}

/** Blocks the UI behind a biometric/device-credential prompt when enabled. */
@Composable
private fun BiometricGate(content: @Composable () -> Unit) {
    val context = LocalContext.current
    var unlocked by remember { mutableStateOf(false) }
    var failed by remember { mutableStateOf(false) }

    if (unlocked) {
        content()
        return
    }

    val activity = context as? FragmentActivity
    Scaffold { padding ->
        Column(
            Modifier.fillMaxSize().padding(padding).padding(24.dp),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text("DuddyNet is locked", style = MaterialTheme.typography.headlineSmall)
            Button(onClick = {
                if (activity != null && Biometric.canAuthenticate(activity)) {
                    Biometric.prompt(
                        activity,
                        onSuccess = { unlocked = true; failed = false },
                        onFailure = { failed = true },
                    )
                } else {
                    // No biometric available; don't lock the user out permanently.
                    unlocked = true
                }
            }, modifier = Modifier.padding(top = 16.dp)) {
                Text(if (failed) "Try again" else "Unlock")
            }
        }
    }
}
