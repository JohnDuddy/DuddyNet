package net.duddy.duddynet.di

import android.content.Context
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.launch
import net.duddy.duddynet.BuildConfig
import net.duddy.duddynet.data.DuddyNetRepository
import net.duddy.duddynet.data.SettingsStore
import net.duddy.duddynet.data.TokenStore

/**
 * Minimal manual dependency container. We avoid a DI framework (Hilt) to keep the
 * build free of annotation processors and the graph easy to read.
 *
 * One instance is created in [net.duddy.duddynet.DuddyNetApp] and accessed via
 * [from] in Activities/ViewModels.
 */
class ServiceLocator(context: Context) {
    private val appContext = context.applicationContext
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    val tokenStore: TokenStore by lazy { TokenStore(appContext) }
    val settingsStore: SettingsStore by lazy { SettingsStore(appContext) }
    val repository: DuddyNetRepository by lazy {
        DuddyNetRepository(tokenStore, debug = BuildConfig.DEBUG)
    }

    init {
        // Keep the repository's target host in sync with the saved setting.
        scope.launch {
            settingsStore.settings.map { it.agentHost }.collect { host ->
                if (host.isNotBlank()) repository.setHost(host)
            }
        }
    }

    companion object {
        @Volatile
        private var instance: ServiceLocator? = null

        fun init(context: Context): ServiceLocator =
            instance ?: synchronized(this) {
                instance ?: ServiceLocator(context).also { instance = it }
            }

        fun from(context: Context): ServiceLocator =
            instance ?: init(context)
    }
}
