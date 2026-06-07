package net.duddy.duddynet.ui.wol

import android.app.Application
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
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
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import net.duddy.duddynet.data.ApiResult
import net.duddy.duddynet.data.Device
import net.duddy.duddynet.ui.AppViewModel
import net.duddy.duddynet.ui.components.SectionCard

data class WolState(
    val loading: Boolean = false,
    val devices: List<Device> = emptyList(),
    val busyId: String? = null,
    val results: Map<String, String> = emptyMap(),
    val canWake: Boolean = false,
    val error: String? = null,
)

class WolViewModel(app: Application) : AppViewModel(app) {
    private val _state = MutableStateFlow(WolState())
    val state = _state.asStateFlow()

    init {
        val s = repo.scopes
        _state.update { it.copy(canWake = s.contains("wol") || s.contains("full-admin")) }
    }

    fun load() {
        _state.update { it.copy(loading = true, error = null) }
        viewModelScope.launch {
            when (val r = repo.listDevices(includeHidden = true)) {
                is ApiResult.Success ->
                    _state.update { it.copy(loading = false, devices = r.value.filter { d -> d.macAddress.isNotBlank() }) }
                is ApiResult.HttpError -> _state.update { it.copy(loading = false, error = "Error ${r.status}.") }
                is ApiResult.NetworkError -> _state.update { it.copy(loading = false, error = "Agent unreachable.") }
            }
        }
    }

    fun wake(device: Device) {
        _state.update { it.copy(busyId = device.id) }
        viewModelScope.launch {
            val msg = when (val r = repo.wakeDevice(device.id)) {
                is ApiResult.Success -> if (r.value.success) "Sent to ${r.value.sentTo}" else "Failed: ${r.value.message}"
                is ApiResult.HttpError -> if (r.forbidden) "Token lacks 'wol' scope" else "Failed (${r.status})"
                is ApiResult.NetworkError -> "Agent unreachable"
            }
            _state.update { it.copy(busyId = null, results = it.results + (device.id to msg)) }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun WolScreen(
    contentPadding: PaddingValues,
    vm: WolViewModel = viewModel(),
) {
    val state by vm.state.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { vm.load() }

    Scaffold(topBar = { TopAppBar(title = { Text("Wake-on-LAN") }) }) { inner ->
        LazyColumn(
            Modifier.fillMaxWidth().padding(inner),
            contentPadding = PaddingValues(bottom = contentPadding.calculateBottomPadding() + 16.dp),
        ) {
            item {
                SectionCard {
                    Text(
                        "The home agent sends the magic packet from inside your LAN. The target " +
                            "must have Wake-on-LAN enabled in its BIOS/UEFI and NIC settings.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            state.error?.let { err -> item { SectionCard { Text(err, color = MaterialTheme.colorScheme.error) } } }
            if (!state.canWake) {
                item {
                    SectionCard {
                        Text(
                            "This device's token does not include the 'wol' scope. Re-pair with " +
                                "WOL enabled to wake devices.",
                            color = MaterialTheme.colorScheme.error,
                        )
                    }
                }
            }
            if (state.devices.isEmpty() && !state.loading) {
                item { SectionCard { Text("No devices have a MAC address configured.", color = MaterialTheme.colorScheme.onSurfaceVariant) } }
            }
            items(state.devices, key = { it.id }) { d ->
                SectionCard {
                    Row(
                        Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(Modifier.padding(end = 8.dp)) {
                            Text(d.name, fontWeight = FontWeight.SemiBold, style = MaterialTheme.typography.titleSmall)
                            Text(d.macAddress, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                            state.results[d.id]?.let {
                                Text(it, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.primary)
                            }
                        }
                        Button(onClick = { vm.wake(d) }, enabled = state.canWake && state.busyId != d.id) {
                            Text(if (state.busyId == d.id) "Waking…" else "Wake")
                        }
                    }
                }
            }
        }
    }
}
