package net.duddy.duddynet.ui.devices

import android.app.Application
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
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
import net.duddy.duddynet.data.DeviceInput
import net.duddy.duddynet.ui.AppViewModel
import net.duddy.duddynet.ui.components.SectionCard
import net.duddy.duddynet.util.Time

data class DevicesState(
    val loading: Boolean = false,
    val devices: List<Device> = emptyList(),
    val includeHidden: Boolean = false,
    val error: String? = null,
    val canWrite: Boolean = true,
)

class DevicesViewModel(app: Application) : AppViewModel(app) {
    private val _state = MutableStateFlow(DevicesState())
    val state = _state.asStateFlow()

    init {
        val s = repo.scopes
        _state.update { it.copy(canWrite = s.contains("device-admin") || s.contains("full-admin")) }
    }

    fun load() {
        _state.update { it.copy(loading = true, error = null) }
        viewModelScope.launch {
            when (val r = repo.listDevices(includeHidden = _state.value.includeHidden)) {
                is ApiResult.Success -> _state.update { it.copy(loading = false, devices = r.value) }
                is ApiResult.HttpError -> _state.update { it.copy(loading = false, error = "Error ${r.status}: ${r.message}") }
                is ApiResult.NetworkError -> _state.update { it.copy(loading = false, error = "Agent unreachable.") }
            }
        }
    }

    fun toggleIncludeHidden() {
        _state.update { it.copy(includeHidden = !it.includeHidden) }
        load()
    }

    fun save(existingId: String?, input: DeviceInput, onDone: () -> Unit) {
        viewModelScope.launch {
            val r = if (existingId == null) repo.createDevice(input) else repo.updateDevice(existingId, input)
            when (r) {
                is ApiResult.Success -> { load(); onDone() }
                is ApiResult.HttpError -> _state.update { it.copy(error = "Save failed (${r.status}): ${r.message}") }
                is ApiResult.NetworkError -> _state.update { it.copy(error = "Agent unreachable.") }
            }
        }
    }

    fun setCritical(d: Device, value: Boolean) =
        save(d.id, DeviceInput(critical = value)) {}

    fun setHidden(d: Device, value: Boolean) =
        save(d.id, DeviceInput(hidden = value)) {}
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DevicesScreen(
    contentPadding: PaddingValues,
    onOpenDevice: (String) -> Unit,
    vm: DevicesViewModel = viewModel(),
) {
    val state by vm.state.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { vm.load() }

    var editing by remember { mutableStateOf<Device?>(null) }
    var showDialog by remember { mutableStateOf(false) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Devices") },
                actions = {
                    TextButton(onClick = vm::toggleIncludeHidden) {
                        Text(if (state.includeHidden) "Hide hidden" else "Show hidden")
                    }
                },
            )
        },
        floatingActionButton = {
            if (state.canWrite) {
                FloatingActionButton(onClick = { editing = null; showDialog = true }) {
                    Icon(Icons.Filled.Add, contentDescription = "Add device")
                }
            }
        },
    ) { inner ->
        LazyColumn(
            Modifier.fillMaxWidth().padding(inner),
            contentPadding = PaddingValues(bottom = contentPadding.calculateBottomPadding() + 80.dp),
        ) {
            state.error?.let { err ->
                item { SectionCard { Text(err, color = MaterialTheme.colorScheme.error) } }
            }
            if (state.devices.isEmpty() && !state.loading) {
                item {
                    SectionCard {
                        Text(
                            "No devices configured. Tap + to add your router, NAS, or PC. " +
                                "You can also run `duddynet-agent scan-lan` on the home server to discover hosts.",
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }
            items(state.devices, key = { it.id }) { d ->
                DeviceListItem(
                    device = d,
                    canWrite = state.canWrite,
                    onOpen = { onOpenDevice(d.id) },
                    onEdit = { editing = d; showDialog = true },
                    onToggleCritical = { vm.setCritical(d, !d.critical) },
                    onToggleHidden = { vm.setHidden(d, !d.hidden) },
                )
            }
        }
    }

    if (showDialog) {
        DeviceDialog(
            existing = editing,
            onDismiss = { showDialog = false },
            onSave = { input -> vm.save(editing?.id, input) { showDialog = false } },
        )
    }
}

@Composable
private fun DeviceListItem(
    device: Device,
    canWrite: Boolean,
    onOpen: () -> Unit,
    onEdit: () -> Unit,
    onToggleCritical: () -> Unit,
    onToggleHidden: () -> Unit,
) {
    var menu by remember { mutableStateOf(false) }
    SectionCard {
        Row(
            Modifier.fillMaxWidth().clickable(onClick = onOpen),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(Modifier.padding(end = 8.dp)) {
                Text(
                    (if (device.critical) "★ " else "") + device.name + (if (device.hidden) " (hidden)" else ""),
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    "${device.type} · ${device.lanIp.ifBlank { device.tailnetName.ifBlank { "no address" } }}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Text(
                    "Last seen: ${Time.relative(device.lastSeenAt)}",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            if (canWrite) {
                IconButton(onClick = { menu = true }) {
                    Icon(Icons.Filled.MoreVert, contentDescription = "Device options")
                }
                DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                    DropdownMenuItem(text = { Text("Edit") }, onClick = { menu = false; onEdit() })
                    DropdownMenuItem(
                        text = { Text(if (device.critical) "Unmark critical" else "Mark critical") },
                        onClick = { menu = false; onToggleCritical() },
                    )
                    DropdownMenuItem(
                        text = { Text(if (device.hidden) "Unhide" else "Hide") },
                        onClick = { menu = false; onToggleHidden() },
                    )
                }
            }
        }
    }
}

@Composable
private fun DeviceDialog(
    existing: Device?,
    onDismiss: () -> Unit,
    onSave: (DeviceInput) -> Unit,
) {
    var name by remember { mutableStateOf(existing?.name ?: "") }
    var type by remember { mutableStateOf(existing?.type ?: "other") }
    var lanIp by remember { mutableStateOf(existing?.lanIp ?: "") }
    var tailnet by remember { mutableStateOf(existing?.tailnetName ?: "") }
    var mac by remember { mutableStateOf(existing?.macAddress ?: "") }
    var ports by remember { mutableStateOf(existing?.allowedPorts?.joinToString(",") ?: "") }
    var notes by remember { mutableStateOf(existing?.notes ?: "") }

    AlertDialog(
        onDismissRequest = onDismiss,
        confirmButton = {
            TextButton(onClick = {
                val portList = ports.split(",").mapNotNull { it.trim().toIntOrNull() }
                onSave(
                    DeviceInput(
                        name = name.trim(),
                        type = type.trim().ifBlank { "other" },
                        lanIp = lanIp.trim(),
                        tailnetName = tailnet.trim(),
                        macAddress = mac.trim(),
                        allowedPorts = portList,
                        notes = notes.trim(),
                    ),
                )
            }) { Text("Save") }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text("Cancel") } },
        title = { Text(if (existing == null) "Add device" else "Edit device") },
        text = {
            Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(name, { name = it }, label = { Text("Name") }, singleLine = true)
                OutlinedTextField(type, { type = it }, label = { Text("Type (router/nas/pc/pi/...)") }, singleLine = true)
                OutlinedTextField(lanIp, { lanIp = it }, label = { Text("LAN IP") }, singleLine = true)
                OutlinedTextField(tailnet, { tailnet = it }, label = { Text("Tailnet hostname (optional)") }, singleLine = true)
                OutlinedTextField(mac, { mac = it }, label = { Text("MAC address (for WOL)") }, singleLine = true)
                OutlinedTextField(ports, { ports = it }, label = { Text("Allowed ports (comma-separated)") }, singleLine = true)
                OutlinedTextField(notes, { notes = it }, label = { Text("Notes") })
            }
        },
    )
}
