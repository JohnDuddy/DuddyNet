package net.duddy.duddynet.ui.dashboard

import android.app.Application
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.viewmodel.compose.viewModel
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import net.duddy.duddynet.data.ApiResult
import net.duddy.duddynet.data.Device
import net.duddy.duddynet.ui.AppViewModel
import net.duddy.duddynet.ui.components.LabeledRow
import net.duddy.duddynet.ui.components.SectionCard
import net.duddy.duddynet.ui.components.Status
import net.duddy.duddynet.ui.components.StatusBadge
import net.duddy.duddynet.util.Time

data class DeviceStatus(val device: Device, val status: Status, val latencyMs: Long?)

data class DashboardState(
    val loading: Boolean = false,
    val agentReachable: Boolean? = null,
    val tailscaleStatus: String = "unknown",
    val lanReachable: Boolean? = null,
    val lastSync: String? = null,
    val devices: List<DeviceStatus> = emptyList(),
    val error: String? = null,
    val unauthorized: Boolean = false,
)

class DashboardViewModel(app: Application) : AppViewModel(app) {
    private val _state = MutableStateFlow(DashboardState())
    val state = _state.asStateFlow()

    fun refresh() {
        _state.update { it.copy(loading = true, error = null, unauthorized = false) }
        viewModelScope.launch {
            // 1) Agent + Tailscale reachability.
            when (val h = repo.health()) {
                is ApiResult.Success ->
                    _state.update { it.copy(agentReachable = true, tailscaleStatus = h.value.tailscaleStatus) }
                is ApiResult.HttpError ->
                    _state.update { it.copy(agentReachable = true, error = "Agent error ${h.status}: ${h.message}") }
                is ApiResult.NetworkError ->
                    _state.update { it.copy(agentReachable = false, tailscaleStatus = "unreachable") }
            }

            // 2) Devices + per-device checks (parallel).
            when (val list = repo.listDevices()) {
                is ApiResult.Success -> {
                    val statuses = list.value.map { d ->
                        async {
                            when (val c = repo.checkDevice(d.id)) {
                                is ApiResult.Success ->
                                    DeviceStatus(d, Status.fromHealth(c.value.online, c.value.latencyMs), c.value.latencyMs)
                                else -> DeviceStatus(d, Status.UNKNOWN, null)
                            }
                        }
                    }.awaitAll()
                    val anyOnline = statuses.any { it.status == Status.ONLINE || it.status == Status.SLOW }
                    _state.update {
                        it.copy(
                            loading = false,
                            devices = statuses.sortedByDescending { s -> s.device.critical },
                            lanReachable = if (statuses.isEmpty()) null else anyOnline,
                            lastSync = Time.relativeMs(System.currentTimeMillis()),
                        )
                    }
                }
                is ApiResult.HttpError ->
                    _state.update {
                        it.copy(
                            loading = false,
                            unauthorized = list.unauthorized,
                            error = if (list.unauthorized) "Access was revoked or expired. Re-pair in Settings."
                            else "Could not load devices (${list.status}).",
                        )
                    }
                is ApiResult.NetworkError ->
                    _state.update { it.copy(loading = false, error = "Agent unreachable over Tailscale.") }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DashboardScreen(
    contentPadding: PaddingValues,
    onOpenDevice: (String) -> Unit,
    onOpenServices: () -> Unit,
    onOpenFiles: () -> Unit,
    vm: DashboardViewModel = viewModel(),
) {
    val state by vm.state.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { vm.refresh() }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("DuddyNet") },
                actions = {
                    IconButton(onClick = vm::refresh) {
                        Icon(Icons.Filled.Refresh, contentDescription = "Refresh")
                    }
                },
            )
        },
    ) { inner ->
        LazyColumn(
            Modifier.fillMaxWidth().padding(inner),
            contentPadding = PaddingValues(bottom = contentPadding.calculateBottomPadding() + 16.dp),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            item {
                SectionCard("Connectivity") {
                    ReachRow("Tailscale", reachToStatus(state.tailscaleStatus == "running"))
                    ReachRow("Home agent", reachToStatus(state.agentReachable))
                    ReachRow("Home LAN", reachToStatus(state.lanReachable))
                    LabeledRow("Last sync", state.lastSync ?: "—")
                    if (state.loading) {
                        Spacer(Modifier.padding(2.dp))
                        Text("Refreshing…", style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
            }

            state.error?.let { err ->
                item {
                    SectionCard {
                        Text(err, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium)
                    }
                }
            }

            item {
                Text(
                    "Devices",
                    Modifier.padding(start = 20.dp, top = 12.dp, bottom = 4.dp),
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                )
            }

            if (state.devices.isEmpty() && !state.loading) {
                item {
                    SectionCard {
                        Text(
                            "No devices yet. Add your router, NAS, and PCs from the Devices tab.",
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }

            items(state.devices, key = { it.device.id }) { ds ->
                SectionCard {
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .clickable { onOpenDevice(ds.device.id) },
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column {
                            val name = if (ds.device.critical) "★ ${ds.device.name}" else ds.device.name
                            Text(name, style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
                            Text(
                                "${ds.device.type} · ${ds.device.lanIp.ifBlank { ds.device.tailnetName }}",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                        StatusBadge(ds.status)
                    }
                }
            }
        }
    }
}

@Composable
private fun ReachRow(label: String, status: Status) {
    Row(
        Modifier.fillMaxWidth().padding(vertical = 4.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, style = MaterialTheme.typography.bodyMedium)
        StatusBadge(status)
    }
}

private fun reachToStatus(reachable: Boolean?): Status = when (reachable) {
    true -> Status.ONLINE
    false -> Status.OFFLINE
    null -> Status.UNKNOWN
}
