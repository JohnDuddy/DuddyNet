package net.duddy.duddynet.ui.files

import android.app.Application
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.widget.Toast
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
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
import net.duddy.duddynet.ui.AppViewModel
import net.duddy.duddynet.ui.components.SectionCard

class FilesViewModel(app: Application) : AppViewModel(app) {
    private val _devices = MutableStateFlow<List<Device>>(emptyList())
    val devices = _devices.asStateFlow()

    fun load() {
        viewModelScope.launch {
            (repo.listDevices(includeHidden = true) as? ApiResult.Success)?.let { r ->
                _devices.update { r.value.filter { d -> d.type == "nas" || d.type == "server" } }
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FilesScreen(
    onBack: () -> Unit,
    vm: FilesViewModel = viewModel(),
) {
    val devices by vm.devices.collectAsStateWithLifecycle()
    val context = LocalContext.current
    LaunchedEffect(Unit) { vm.load() }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("File access") },
                navigationIcon = {
                    IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back") }
                },
            )
        },
    ) { inner ->
        LazyColumn(Modifier.fillMaxSize().padding(inner)) {
            item {
                SectionCard {
                    Text(
                        "DuddyNet doesn't embed an SMB client. Connect to your NAS shares directly " +
                            "over Tailscale using the addresses below — your NAS credentials are entered " +
                            "in your file manager, never stored in this app.",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
            if (devices.isEmpty()) {
                item { SectionCard { Text("No NAS/server devices configured.", color = MaterialTheme.colorScheme.onSurfaceVariant) } }
            }
            items(devices, key = { it.id }) { d ->
                val host = d.lanIp.ifBlank { d.tailnetName }
                SectionCard(d.name) {
                    CopyLine(context, "Windows UNC", "\\\\$host\\")
                    CopyLine(context, "Android (file manager)", "smb://$host/")
                    Text(
                        "Tip: in Android's Files app or an SMB-capable app (e.g. CX File Explorer), " +
                            "add a network location pointing at smb://$host/ and sign in with your NAS account.",
                        Modifier.padding(top = 8.dp),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }
    }
}

@Composable
private fun CopyLine(context: Context, label: String, value: String) {
    Column(Modifier.fillMaxWidth().padding(vertical = 4.dp)) {
        Text(label, style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Text(value, fontWeight = FontWeight.Medium, style = MaterialTheme.typography.bodyMedium)
        TextButton(onClick = {
            val cm = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
            cm.setPrimaryClip(ClipData.newPlainText(label, value))
            Toast.makeText(context, "$label copied", Toast.LENGTH_SHORT).show()
        }) { Text("Copy") }
    }
}
