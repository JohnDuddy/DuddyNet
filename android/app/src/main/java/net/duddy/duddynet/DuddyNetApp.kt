package net.duddy.duddynet

import android.app.Application
import net.duddy.duddynet.di.ServiceLocator

class DuddyNetApp : Application() {
    lateinit var container: ServiceLocator
        private set

    override fun onCreate() {
        super.onCreate()
        container = ServiceLocator.init(this)
    }
}
