package net.duddy.duddynet.ui.onboarding

import android.app.Application
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewModelScope
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import net.duddy.duddynet.data.ApiResult
import net.duddy.duddynet.ui.AppViewModel

data class OnboardingState(
    val host: String = "",
    val code: String = "",
    val label: String = "My Phone",
    val loading: Boolean = false,
    val testResult: String? = null,
    val error: String? = null,
)

class OnboardingViewModel(app: Application) : AppViewModel(app) {
    private val _state = MutableStateFlow(OnboardingState())
    val state = _state.asStateFlow()

    fun onHost(v: String) = _state.update { it.copy(host = v, error = null) }
    fun onCode(v: String) = _state.update { it.copy(code = v, error = null) }
    fun onLabel(v: String) = _state.update { it.copy(label = v) }

    fun testConnection() {
        val host = _state.value.host.trim()
        if (host.isEmpty()) {
            _state.update { it.copy(error = "Enter the agent host first") }
            return
        }
        _state.update { it.copy(loading = true, testResult = null, error = null) }
        repo.setHost(host)
        viewModelScope.launch {
            when (val r = repo.health()) {
                is ApiResult.Success ->
                    _state.update {
                        it.copy(
                            loading = false,
                            testResult = "Reached ${r.value.service} v${r.value.version} " +
                                "on ${r.value.hostname} (Tailscale: ${r.value.tailscaleStatus})",
                        )
                    }
                is ApiResult.HttpError ->
                    _state.update { it.copy(loading = false, error = "Agent responded ${r.status}: ${r.message}") }
                is ApiResult.NetworkError ->
                    _state.update {
                        it.copy(
                            loading = false,
                            error = "Could not reach the agent. Is Tailscale connected and the host correct?",
                        )
                    }
            }
        }
    }

    fun pair(onPaired: () -> Unit) {
        val s = _state.value
        if (s.host.isBlank() || s.code.isBlank()) {
            _state.update { it.copy(error = "Host and pairing code are required") }
            return
        }
        _state.update { it.copy(loading = true, error = null) }
        viewModelScope.launch {
            when (val r = repo.pair(s.host.trim(), s.code.trim(), s.label.trim())) {
                is ApiResult.Success -> {
                    settings.setAgentHost(s.host.trim())
                    _state.update { it.copy(loading = false) }
                    onPaired()
                }
                is ApiResult.HttpError ->
                    _state.update {
                        it.copy(
                            loading = false,
                            error = if (r.status == 401) "Pairing code is invalid or expired"
                            else "Pairing failed (${r.status}): ${r.message}",
                        )
                    }
                is ApiResult.NetworkError ->
                    _state.update { it.copy(loading = false, error = "Could not reach the agent over Tailscale.") }
            }
        }
    }
}

@Composable
fun OnboardingScreen(
    onPaired: () -> Unit,
    vm: OnboardingViewModel = viewModel(),
) {
    val state by vm.state.collectAsStateWithLifecycle()

    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(20.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text("DuddyNet", style = MaterialTheme.typography.headlineMedium, fontWeight = FontWeight.Bold)
        Text(
            "Secure remote access to your home network over Tailscale.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )

        Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.primaryContainer)) {
            Text(
                "Before pairing: install the Tailscale app, sign in, and make sure it's " +
                    "Connected. DuddyNet uses your existing Tailscale connection — it doesn't " +
                    "control the VPN itself.",
                modifier = Modifier.padding(16.dp),
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onPrimaryContainer,
            )
        }

        OutlinedTextField(
            value = state.host,
            onValueChange = vm::onHost,
            label = { Text("Agent host") },
            placeholder = { Text("duddynet-agent.tailXXXX.ts.net or 100.x.y.z") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = state.code,
            onValueChange = vm::onCode,
            label = { Text("Pairing code") },
            placeholder = { Text("K7QM-29FB-XTRP") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = state.label,
            onValueChange = vm::onLabel,
            label = { Text("This device's label") },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )

        OutlinedButton(
            onClick = vm::testConnection,
            enabled = !state.loading,
            modifier = Modifier.fillMaxWidth(),
        ) { Text("Test connection") }

        Button(
            onClick = { vm.pair(onPaired) },
            enabled = !state.loading,
            modifier = Modifier.fillMaxWidth(),
        ) { Text("Pair") }

        if (state.loading) {
            Spacer(Modifier.height(4.dp))
            CircularProgressIndicator()
        }
        state.testResult?.let {
            Text(it, color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.bodyMedium)
        }
        state.error?.let {
            Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodyMedium)
        }
    }
}
