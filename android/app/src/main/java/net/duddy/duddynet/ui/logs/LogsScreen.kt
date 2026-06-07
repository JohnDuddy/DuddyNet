package net.duddy.duddynet.ui.logs

import android.app.Application
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
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
import net.duddy.duddynet.data.AuditLogEntry
import net.duddy.duddynet.data.DuddyNetRepository
import net.duddy.duddynet.ui.AppViewModel
import net.duddy.duddynet.ui.components.SectionCard
import net.duddy.duddynet.util.Time

data class LogsState(
    val loading: Boolean = false,
    val agentLogs: List<AuditLogEntry> = emptyList(),
    val canReadAgentLogs: Boolean = false,
    val onlyFailures: Boolean = false,
    val error: String? = null,
)

class LogsViewModel(app: Application) : AppViewModel(app) {
    private val _state = MutableStateFlow(LogsState())
    val state = _state.asStateFlow()

    val localLog get() = repo.localLog

    init {
        val s = repo.scopes
        _state.update { it.copy(canReadAgentLogs = s.contains("logs") || s.contains("full-admin")) }
    }

    fun load() {
        if (!_state.value.canReadAgentLogs) return
        _state.update { it.copy(loading = true, error = null) }
        viewModelScope.launch {
            val success = if (_state.value.onlyFailures) false else null
            when (val r = repo.logs(success = success, limit = 200)) {
                is ApiResult.Success -> _state.update { it.copy(loading = false, agentLogs = r.value.items) }
                is ApiResult.HttpError -> _state.update { it.copy(loading = false, error = "Error ${r.status}: ${r.message}") }
                is ApiResult.NetworkError -> _state.update { it.copy(loading = false, error = "Agent unreachable.") }
            }
        }
    }

    fun toggleFailures() {
        _state.update { it.copy(onlyFailures = !it.onlyFailures) }
        load()
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LogsScreen(
    contentPadding: PaddingValues,
    vm: LogsViewModel = viewModel(),
) {
    val state by vm.state.collectAsStateWithLifecycle()
    val local by vm.localLog.collectAsStateWithLifecycle()
    LaunchedEffect(Unit) { vm.load() }

    Scaffold(topBar = { TopAppBar(title = { Text("Logs") }) }) { inner ->
        LazyColumn(
            Modifier.fillMaxWidth().padding(inner),
            contentPadding = PaddingValues(bottom = contentPadding.calculateBottomPadding() + 16.dp),
        ) {
            item {
                Row(Modifier.padding(horizontal = 16.dp, vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    FilterChip(selected = state.onlyFailures, onClick = vm::toggleFailures, label = { Text("Failures only") })
                }
            }

            item { Header("Agent audit log") }
            state.error?.let { err -> item { SectionCard { Text(err, color = MaterialTheme.colorScheme.error) } } }
            if (!state.canReadAgentLogs) {
                item {
                    SectionCard {
                        Text(
                            "This device's token can't read the agent audit log (needs the 'logs' scope).",
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            } else if (state.agentLogs.isEmpty() && !state.loading) {
                item { SectionCard { Text("No matching audit entries.", color = MaterialTheme.colorScheme.onSurfaceVariant) } }
            }
            items(state.agentLogs, key = { it.id }) { e ->
                SectionCard {
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        Text(e.action, fontWeight = FontWeight.SemiBold, style = MaterialTheme.typography.titleSmall)
                        Text(if (e.success) "ok" else "fail", color = if (e.success) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error)
                    }
                    Text(
                        "${Time.relative(e.timestamp)} · ${e.targetType}${if (e.targetId.isNotBlank()) "/" + e.targetId.take(8) else ""} · ${e.remoteIp}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    if (e.detail.isNotBlank()) {
                        Text(e.detail, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
            }

            item { Header("This app's activity") }
            if (local.isEmpty()) {
                item { SectionCard { Text("No local activity yet.", color = MaterialTheme.colorScheme.onSurfaceVariant) } }
            }
            items(local) { entry ->
                SectionCard {
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        Text(entry.message, style = MaterialTheme.typography.bodyMedium)
                        Text(if (entry.success) "ok" else "fail", color = if (entry.success) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.error)
                    }
                    Text(Time.relativeMs(entry.timestampMs), style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
            }
        }
    }
}

@Composable
private fun Header(text: String) {
    Text(
        text,
        Modifier.padding(start = 20.dp, top = 14.dp, bottom = 2.dp),
        style = MaterialTheme.typography.titleMedium,
        fontWeight = FontWeight.SemiBold,
    )
}
