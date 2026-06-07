package net.duddy.duddynet.ui.settings

import android.app.Application
import android.content.Intent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.compose.viewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import net.duddy.duddynet.BuildConfig
import net.duddy.duddynet.data.SettingsStore
import net.duddy.duddynet.ui.AppViewModel
import net.duddy.duddynet.ui.components.SectionCard

class SettingsViewModel(app: Application) : AppViewModel(app) {
    val settingsFlow get() = settings.settings

    private val _tokenInfo = MutableStateFlow(
        TokenInfo(tokens.tokenId, tokens.scopes.toList(), tokens.hasToken),
    )
    val tokenInfo = _tokenInfo.asStateFlow()

    data class TokenInfo(val id: String?, val scopes: List<String>, val present: Boolean)

    fun setHost(v: String) = viewModelScope.launch { settings.setAgentHost(v); repo.setHost(v) }
    fun setRefresh(v: Int) = viewModelScope.launch { settings.setRefreshInterval(v) }
    fun setDark(mode: SettingsStore.DarkMode) = viewModelScope.launch { settings.setDarkMode(mode) }
    fun setReadOnly(v: Boolean) = viewModelScope.launch { settings.setReadOnly(v) }
    fun setBiometric(v: Boolean) = viewModelScope.launch { settings.setRequireBiometric(v) }

    fun clearLocalData(onDone: () -> Unit) = viewModelScope.launch {
        repo.signOut()
        settings.clear()
        onDone()
    }

    fun diagnostics(host: String): String = buildString {
        appendLine("DuddyNet diagnostics (redacted)")
        appendLine("app_version: ${BuildConfig.VERSION_NAME}")
        appendLine("agent_host: $host")
        appendLine("base_url: ${repo.baseUrl()}")
        appendLine("token_present: ${tokens.hasToken}")
        appendLine("token_id: ${tokens.tokenId ?: "none"}")
        appendLine("token_scopes: ${tokens.scopes.joinToString(",")}")
        appendLine("note: token VALUE is intentionally omitted.")
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    contentPadding: PaddingValues,
    onSignedOut: () -> Unit,
    onOpenServices: () -> Unit,
    onOpenFiles: () -> Unit,
    vm: SettingsViewModel = viewModel(),
) {
    val settings by vm.settingsFlow.collectAsStateWithLifecycle(SettingsStore.Settings())
    val tokenInfo by vm.tokenInfo.collectAsStateWithLifecycle()
    val context = LocalContext.current

    var host by remember(settings.agentHost) { mutableStateOf(settings.agentHost) }

    Scaffold(topBar = { TopAppBar(title = { Text("Settings") }) }) { inner ->
        Column(
            Modifier
                .fillMaxSize()
                .padding(inner)
                .verticalScroll(rememberScrollState())
                .padding(bottom = contentPadding.calculateBottomPadding()),
        ) {
            SectionCard("Agent") {
                OutlinedTextField(
                    value = host,
                    onValueChange = { host = it },
                    label = { Text("Agent host") },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Button(onClick = { vm.setHost(host) }, modifier = Modifier.padding(top = 8.dp)) { Text("Save host") }
            }

            SectionCard("App token") {
                Text("Token present: ${tokenInfo.present}")
                Text("Token id: ${tokenInfo.id ?: "none"}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text("Scopes: ${tokenInfo.scopes.joinToString(", ").ifBlank { "—" }}", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                Text(
                    "To revoke this device, run `duddynet-agent revoke-token ${tokenInfo.id ?: "<id>"}` on the home server.",
                    Modifier.padding(top = 6.dp),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }

            SectionCard("Preferences") {
                ToggleRow("Read-only mode (hide write actions)", settings.readOnly, vm::setReadOnly)
                ToggleRow("Require biometric unlock", settings.requireBiometric, vm::setBiometric)
                Text("Refresh interval", Modifier.padding(top = 8.dp), fontWeight = FontWeight.Medium)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    listOf(30, 60, 120, 300).forEach { secs ->
                        FilterChip(
                            selected = settings.refreshIntervalSeconds == secs,
                            onClick = { vm.setRefresh(secs) },
                            label = { Text("${secs}s") },
                        )
                    }
                }
                Text("Theme", Modifier.padding(top = 8.dp), fontWeight = FontWeight.Medium)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    SettingsStore.DarkMode.entries.forEach { mode ->
                        FilterChip(
                            selected = settings.darkMode == mode,
                            onClick = { vm.setDark(mode) },
                            label = { Text(mode.name.lowercase()) },
                        )
                    }
                }
            }

            SectionCard("Shortcuts") {
                OutlinedButton(onClick = onOpenServices, modifier = Modifier.fillMaxWidth()) { Text("Services") }
                OutlinedButton(onClick = onOpenFiles, modifier = Modifier.fillMaxWidth().padding(top = 8.dp)) { Text("File access helper") }
            }

            SectionCard("Maintenance") {
                OutlinedButton(
                    onClick = {
                        val text = vm.diagnostics(host)
                        val send = Intent(Intent.ACTION_SEND).apply {
                            type = "text/plain"
                            putExtra(Intent.EXTRA_SUBJECT, "DuddyNet diagnostics")
                            putExtra(Intent.EXTRA_TEXT, text)
                        }
                        context.startActivity(Intent.createChooser(send, "Export diagnostics"))
                    },
                    modifier = Modifier.fillMaxWidth(),
                ) { Text("Export diagnostics (redacted)") }

                Button(
                    onClick = { vm.clearLocalData(onSignedOut) },
                    modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                ) { Text("Clear local data & sign out") }
            }

            SectionCard("About") {
                Text("DuddyNet ${BuildConfig.VERSION_NAME}", fontWeight = FontWeight.SemiBold)
                Text(
                    "Private remote access to your home network over Tailscale. No exposed ports, " +
                        "no device passwords stored on this phone.",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
    }
}

@Composable
private fun ToggleRow(label: String, checked: Boolean, onChange: (Boolean) -> Unit) {
    Row(
        Modifier.fillMaxWidth().padding(vertical = 4.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, Modifier.padding(end = 12.dp))
        Switch(checked = checked, onCheckedChange = onChange)
    }
}
