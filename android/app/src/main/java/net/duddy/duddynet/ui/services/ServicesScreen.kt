package net.duddy.duddynet.ui.services

import android.app.Application
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
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
import net.duddy.duddynet.data.Service
import net.duddy.duddynet.ui.AppViewModel
import net.duddy.duddynet.ui.components.SectionCard
import net.duddy.duddynet.util.CustomTabs

data class ServicesState(
    val loading: Boolean = false,
    val devicesById: Map<String, Device> = emptyMap(),
    val services: List<Service> = emptyList(),
    val error: String? = null,
)

class ServicesViewModel(app: Application) : AppViewModel(app) {
    private val _state = MutableStateFlow(ServicesState())
    val state = _state.asStateFlow()

    fun load() {
        _state.update { it.copy(loading = true, error = null) }
        viewModelScope.launch {
            val devices = (repo.listDevices(includeHidden = true) as? ApiResult.Success)?.value ?: emptyList()
            when (val s = repo.listServices()) {
                is ApiResult.Success ->
                    _state.update {
                        it.copy(loading = false, services = s.value, devicesById = devices.associateBy { d -> d.id })
                    }
                is ApiResult.HttpError -> _state.update { it.copy(loading = false, error = "Error ${s.status}: ${s.message}") }
                is ApiResult.NetworkError -> _state.update { it.copy(loading = false, error = "Agent unreachable.") }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ServicesScreen(
    contentPadding: PaddingValues,
    onBack: () -> Unit,
    vm: ServicesViewModel = viewModel(),
) {
    val state by vm.state.collectAsStateWithLifecycle()
    val context = LocalContext.current
    LaunchedEffect(Unit) { vm.load() }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Services") },
                navigationIcon = {
                    IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back") }
                },
            )
        },
    ) { inner ->
        val grouped = state.services.groupBy { it.deviceId }
        LazyColumn(
            Modifier.fillMaxWidth().padding(inner),
            contentPadding = PaddingValues(bottom = contentPadding.calculateBottomPadding() + 16.dp),
        ) {
            state.error?.let { err -> item { SectionCard { Text(err, color = MaterialTheme.colorScheme.error) } } }
            if (state.services.isEmpty() && !state.loading) {
                item { SectionCard { Text("No services configured yet.", color = MaterialTheme.colorScheme.onSurfaceVariant) } }
            }
            grouped.forEach { (deviceId, svcs) ->
                val device = state.devicesById[deviceId]
                item {
                    SectionCard(device?.name ?: "Unknown device") {
                        val address = device?.lanIp?.ifBlank { device.tailnetName } ?: ""
                        svcs.forEach { svc ->
                            Column(Modifier.fillMaxWidth().padding(vertical = 6.dp)) {
                                Text(svc.name, fontWeight = FontWeight.SemiBold, style = MaterialTheme.typography.titleSmall)
                                Text(
                                    "${svc.serviceType} · ${svc.protocol}/${svc.port}${if (svc.readOnly) " · read-only" else ""}" +
                                        if (!svc.enabled) " · disabled" else "",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                                val url = svc.url.ifBlank {
                                    if (address.isBlank()) "" else {
                                        val scheme = if (svc.protocol == "https" || svc.port == 443 || svc.port == 5001) "https" else "http"
                                        if (svc.protocol in listOf("http", "https") || svc.port in listOf(80, 443, 5000, 5001, 8123))
                                            "$scheme://$address:${svc.port}${svc.path}" else ""
                                    }
                                }
                                if (url.isNotBlank()) {
                                    TextButton(onClick = { CustomTabs.open(context, url) }) { Text("Open $url") }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
