package net.duddy.duddynet.ui

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import net.duddy.duddynet.DuddyNetApp
import net.duddy.duddynet.di.ServiceLocator

/**
 * Base class for app ViewModels. Using [AndroidViewModel] lets Compose's default
 * factory construct them (it supplies the Application), so screens can call
 * `viewModel()` without a custom factory. The [container] gives access to the
 * repository and stores.
 */
abstract class AppViewModel(app: Application) : AndroidViewModel(app) {
    protected val container: ServiceLocator get() = (getApplication() as DuddyNetApp).container
    protected val repo get() = container.repository
    protected val settings get() = container.settingsStore
    protected val tokens get() = container.tokenStore
}
