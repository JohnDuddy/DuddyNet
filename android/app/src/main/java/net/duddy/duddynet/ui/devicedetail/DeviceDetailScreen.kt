package net.duddy.duddynet.ui.devicedetail

import android.app.Application
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.widget.Toast
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.compose.viewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import net.duddy.duddynet.data.ApiResult
import net.duddy.duddynet.data.Device
import net.duddy.duddynet.data.HealthStatus
import net.duddy.duddynet.data.Service
import net.duddy.duddynet.ui.AppViewModel
import net.duddy.duddynet.ui.components.LabeledRow
import net.duddy.duddynet.ui.components.LoadingBox
import net.duddy.duddynet.ui.components.SectionCard
import net.duddy.duddynet.ui.components.Status
import net.duddy.duddynet.ui.components.StatusBadge
import net.duddy.duddynet.util.CustomTabs

data class DetailState(
    val loading: Boolean = true,
    val device: Device? = null,
    val services: List<Service> = emptyList(),
    val health: HealthStatus? = null,
    val busy: Boolean = false,
    val message: String? = null,
    val canWake: Boolean = false,
)

class DeviceDetailViewModel(app: Application) : AppViewModel(app) {
    private val _state = MutableStateFlow(DetailState())
    val state = _state.asStateFlow()

    init {
        val s = repo.scopes
        _state.update { it.copy(canWake = s.contains("wol") || s.contains("full-admin")) }
    }

    fun load(deviceId: String) {
        _state.update { it.copy(loading = true) }
        viewModelScope.launch {
            when (val d = repo.getDevice(deviceId)) {
                is ApiResult.Success -> _state.update { it.copy(loading = false, device = d.value) }
                else -> _state.update { it.copy(loading = false, message = "Could not load device.") }
            }
            (repo.listServices(deviceId) as? ApiResult.Success)?.let { s ->
                _state.update { it.copy(services = s.value) }
            }
        }
    }

    fun runCheck() {
        val id = _state.value.device?.id ?: return
        _state.update { it.copy(busy = true, message = null) }
        viewModelScope.launch {
            when (val r = repo.checkDevice(id)) {
                is ApiResult.Success -> _state.update { it.copy(busy = false, health = r.value) }
                is ApiResult.HttpError -> _state.update { it.copy(busy = false, message = "Check failed (${r.status}).") }
                is ApiResult.NetworkError -> _state.update { it.copy(busy = false, message = "Agent unreachable.") }
            }
        }
    }

    fun wake() {
        val id = _state.value.device?.id ?: return
        _state.update { it.copy(busy = true, message = null) }
        viewModelScope.launch {
            when (val r = repo.wakeDevice(id)) {
                is ApiResult.Success ->
                    _state.update { it.copy(busy = false, message = if (r.value.success) "Magic packet sent to ${r.value.sentTo}" else "Wake failed: ${r.value.message}") }
                is ApiResult.HttpError ->
                    _state.update { it.copy(busy = false, message = if (r.forbidden) "Your token lacks the 'wol' scope." else "Wake failed (${r.status}).") }
                is ApiResult.NetworkError ->
                    _state.update { it.copy(busy = false, message = "Agent unreachable.") }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun DeviceDetailScreen(
    deviceId: String,
    onBack: () -> Unit,
    vm: DeviceDetailViewModel = viewModel(),
) {
    val state by vm.state.collectAsStateWithLifecycle()
    val context = LocalContext.current
    LaunchedEffect(deviceId) { vm.load(deviceId) }

    var instruction by remember { mutableStateOf<Pair<String, String>?>(null) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(state.device?.name ?: "Device") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                },
            )
        },
    ) { inner ->
        if (state.loading) {
            LoadingBox(Modifier.padding(inner))
            return@Scaffold
        }
        val device = state.device ?: run {
            Text("Device not found", Modifier.padding(inner).padding(16.dp))
            return@Scaffold
        }
        val address = device.lanIp.ifBlank { device.tailnetName }

        Column(
            Modifier.fillMaxSize().padding(inner).verticalScroll(rememberScrollState()),
        ) {
            SectionCard("Status") {
                val st = Status.fromHealth(state.health?.online, state.health?.latencyMs)
                LabeledRowWithBadge("Reachability", st)
                LabeledRow("Ping latency", state.health?.let { "${it.latencyMs} ms" } ?: "—")
                LabeledRow("Open ports", state.health?.openPorts?.joinToString(",")?.ifBlank { "—" } ?: "—")
                LabeledRow("Type", device.type)
                LabeledRow("LAN IP", device.lanIp.ifBlank { "—" })
                LabeledRow("Tailnet", device.tailnetName.ifBlank { "—" })
                LabeledRow("MAC", device.macAddress.ifBlank { "—" })
                if (device.notes.isNotBlank()) LabeledRow("Notes", device.notes)
            }

            SectionCard("Actions") {
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    Button(onClick = vm::runCheck, enabled = !state.busy) { Text("Run health check") }
                    if (state.canWake && device.macAddress.isNotBlank()) {
                        Button(onClick = vm::wake, enabled = !state.busy) { Text("Wake-on-LAN") }
                    }
                    val webService = state.services.firstOrNull { it.url.isNotBlank() }
                        ?: state.services.firstOrNull { it.protocol == "http" || it.protocol == "https" }
                    val webUrl = webService?.url?.takeIf { it.isNotBlank() } ?: defaultWebUrl(address, state.services)
                    if (webUrl != null) {
                        OutlinedButton(onClick = { CustomTabs.open(context, webUrl) }) { Text("Open web admin") }
                    }
                    OutlinedButton(onClick = { copy(context, "LAN IP", device.lanIp) }) { Text("Copy LAN IP") }
                    if (device.tailnetName.isNotBlank()) {
                        OutlinedButton(onClick = { copy(context, "Tailnet host", device.tailnetName) }) { Text("Copy tailnet host") }
                    }
                    OutlinedButton(onClick = {
                        instruction = "SSH instructions" to "From a terminal on a device that's on your tailnet:\n\n" +
                            "ssh <username>@$address\n\n" +
                            "On Android, use an SSH client app (e.g. Termux/JuiceSSH) and connect to $address:22 " +
                            "over Tailscale. DuddyNet does not embed an SSH client."
                    }) { Text("SSH instructions") }
                    OutlinedButton(onClick = {
                        instruction = "RDP instructions" to "Use a Remote Desktop client over Tailscale:\n\n" +
                            "• Windows: mstsc /v:$address\n" +
                            "• Android: Microsoft 'Remote Desktop' app → add PC → $address:3389\n\n" +
                            "Ensure the target PC has Remote Desktop enabled. DuddyNet does not embed an RDP client."
                    }) { Text("RDP instructions") }
                }
                state.message?.let {
                    Text(it, Modifier.padding(top = 8.dp), color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.bodyMedium)
                }
            }

            SectionCard("Services") {
                if (state.services.isEmpty()) {
                    Text("No services configured for this device.", color = MaterialTheme.colorScheme.onSurfaceVariant)
                } else {
                    state.services.forEach { svc ->
                        Column(Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
                            Text(svc.name, fontWeight = FontWeight.SemiBold, style = MaterialTheme.typography.titleSmall)
                            Text(
                                "${svc.protocol}/${svc.port}${if (svc.readOnly) " · read-only" else ""}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                            val url = svc.url.takeIf { it.isNotBlank() } ?: defaultWebUrlForService(address, svc)
                            if (url != null) {
                                TextButton(onClick = { CustomTabs.open(context, url) }) { Text("Open $url") }
                            }
                        }
                    }
                }
            }
        }
    }

    instruction?.let { (title, body) ->
        AlertDialog(
            onDismissRequest = { instruction = null },
            confirmButton = {
                TextButton(onClick = { copy(context, title, body); instruction = null }) { Text("Copy") }
            },
            dismissButton = { TextButton(onClick = { instruction = null }) { Text("Close") } },
            title = { Text(title) },
            text = { Text(body) },
        )
    }
}

@Composable
private fun LabeledRowWithBadge(label: String, status: Status) {
    androidx.compose.foundation.layout.Row(
        Modifier.fillMaxWidth().padding(vertical = 4.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(label, color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.bodyMedium)
        StatusBadge(status)
    }
}

private fun defaultWebUrl(address: String, services: List<Service>): String? {
    if (address.isBlank()) return null
    return when {
        services.any { it.port == 443 || it.port == 5001 } -> "https://$address"
        services.any { it.port == 80 || it.port == 5000 } -> "http://$address"
        else -> null
    }
}

private fun defaultWebUrlForService(address: String, svc: Service): String? {
    if (address.isBlank()) return null
    val scheme = if (svc.protocol == "https" || svc.port == 443 || svc.port == 5001) "https" else "http"
    return if (svc.protocol == "http" || svc.protocol == "https" || svc.port in listOf(80, 443, 5000, 5001, 8123)) {
        "$scheme://$address:${svc.port}${svc.path}"
    } else {
        null
    }
}

private fun copy(context: Context, label: String, value: String) {
    if (value.isBlank()) {
        Toast.makeText(context, "Nothing to copy", Toast.LENGTH_SHORT).show()
        return
    }
    val cm = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
    cm.setPrimaryClip(ClipData.newPlainText(label, value))
    Toast.makeText(context, "$label copied", Toast.LENGTH_SHORT).show()
}
