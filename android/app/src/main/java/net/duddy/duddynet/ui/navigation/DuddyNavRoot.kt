package net.duddy.duddynet.ui.navigation

import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Icon
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.navigation.NavDestination.Companion.hierarchy
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import net.duddy.duddynet.di.ServiceLocator
import net.duddy.duddynet.ui.dashboard.DashboardScreen
import net.duddy.duddynet.ui.devicedetail.DeviceDetailScreen
import net.duddy.duddynet.ui.devices.DevicesScreen
import net.duddy.duddynet.ui.files.FilesScreen
import net.duddy.duddynet.ui.logs.LogsScreen
import net.duddy.duddynet.ui.onboarding.OnboardingScreen
import net.duddy.duddynet.ui.services.ServicesScreen
import net.duddy.duddynet.ui.settings.SettingsScreen
import net.duddy.duddynet.ui.wol.WolScreen

@Composable
fun DuddyNavRoot() {
    val navController = rememberNavController()
    val context = LocalContext.current
    val container = remember { ServiceLocator.from(context) }

    val start = if (container.repository.hasToken) Routes.DASHBOARD else Routes.ONBOARDING

    val backStack by navController.currentBackStackEntryAsState()
    val currentRoute = backStack?.destination?.route
    val showBar = currentRoute in tabRoutes

    Scaffold(
        bottomBar = {
            if (showBar) {
                NavigationBar {
                    bottomTabs.forEach { tab ->
                        val selected = backStack?.destination?.hierarchy?.any { it.route == tab.route } == true
                        NavigationBarItem(
                            selected = selected,
                            onClick = {
                                navController.navigate(tab.route) {
                                    popUpTo(navController.graph.findStartDestination().id) { saveState = true }
                                    launchSingleTop = true
                                    restoreState = true
                                }
                            },
                            icon = { Icon(tab.icon, contentDescription = tab.label) },
                            label = { Text(tab.label) },
                        )
                    }
                }
            }
        },
    ) { padding ->
        NavHost(
            navController = navController,
            startDestination = start,
            modifier = Modifier,
        ) {
            composable(Routes.ONBOARDING) {
                OnboardingScreen(onPaired = {
                    navController.navigate(Routes.DASHBOARD) {
                        popUpTo(Routes.ONBOARDING) { inclusive = true }
                    }
                })
            }
            composable(Routes.DASHBOARD) {
                DashboardScreen(
                    contentPadding = padding,
                    onOpenDevice = { navController.navigate(Routes.deviceDetail(it)) },
                    onOpenServices = { navController.navigate(Routes.SERVICES) },
                    onOpenFiles = { navController.navigate(Routes.FILES) },
                )
            }
            composable(Routes.DEVICES) {
                DevicesScreen(
                    contentPadding = padding,
                    onOpenDevice = { navController.navigate(Routes.deviceDetail(it)) },
                )
            }
            composable(Routes.DEVICE_DETAIL) { entry ->
                val id = entry.arguments?.getString("id").orEmpty()
                DeviceDetailScreen(deviceId = id, onBack = { navController.popBackStack() })
            }
            composable(Routes.SERVICES) {
                ServicesScreen(contentPadding = padding, onBack = { navController.popBackStack() })
            }
            composable(Routes.WOL) {
                WolScreen(contentPadding = padding)
            }
            composable(Routes.FILES) {
                FilesScreen(onBack = { navController.popBackStack() })
            }
            composable(Routes.LOGS) {
                LogsScreen(contentPadding = padding)
            }
            composable(Routes.SETTINGS) {
                SettingsScreen(
                    contentPadding = padding,
                    onSignedOut = {
                        navController.navigate(Routes.ONBOARDING) {
                            popUpTo(0) { inclusive = true }
                        }
                    },
                    onOpenServices = { navController.navigate(Routes.SERVICES) },
                    onOpenFiles = { navController.navigate(Routes.FILES) },
                )
            }
        }
    }
}
