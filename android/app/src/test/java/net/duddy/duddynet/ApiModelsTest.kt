package net.duddy.duddynet

import kotlinx.serialization.decodeFromString
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import net.duddy.duddynet.data.Device
import net.duddy.duddynet.data.DeviceInput
import net.duddy.duddynet.data.HealthStatus
import net.duddy.duddynet.data.Page
import net.duddy.duddynet.data.WakeResult
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ApiModelsTest {
    private val json = Json {
        ignoreUnknownKeys = true
        encodeDefaults = false
        explicitNulls = false
    }

    @Test
    fun `decodes device from agent json`() {
        val src = """
            {
              "id": "abc",
              "name": "UGREEN NAS",
              "type": "nas",
              "lan_ip": "192.168.1.10",
              "tailnet_name": "nas.tailXXXX.ts.net",
              "mac_address": "AA:BB:CC:DD:EE:FF",
              "allowed_ports": [5000, 5001, 445],
              "notes": "main storage",
              "critical": true,
              "hidden": false,
              "created_at": "2026-06-07T12:00:00Z",
              "updated_at": "2026-06-07T12:00:00Z",
              "last_seen_at": "2026-06-07T12:05:00Z"
            }
        """.trimIndent()
        val d = json.decodeFromString<Device>(src)
        assertEquals("UGREEN NAS", d.name)
        assertEquals("nas", d.type)
        assertEquals(listOf(5000, 5001, 445), d.allowedPorts)
        assertTrue(d.critical)
        assertEquals("2026-06-07T12:05:00Z", d.lastSeenAt)
    }

    @Test
    fun `device input omits null fields`() {
        // A PATCH that only flips 'critical' must not send other fields.
        val encoded = json.encodeToString(DeviceInput(critical = true))
        assertEquals("""{"critical":true}""", encoded)
        assertFalse(encoded.contains("name"))
        assertFalse(encoded.contains("allowed_ports"))
    }

    @Test
    fun `decodes paged devices`() {
        val src = """{"items":[{"id":"1","name":"Router","type":"router"}],"total":1,"limit":1,"offset":0}"""
        val page = json.decodeFromString<Page<Device>>(src)
        assertEquals(1, page.total)
        assertEquals("Router", page.items.first().name)
    }

    @Test
    fun `decodes health status with defaults`() {
        val src = """{"device_id":"x","online":true,"latency_ms":7,"open_ports":[443]}"""
        val hs = json.decodeFromString<HealthStatus>(src)
        assertTrue(hs.online)
        assertEquals(7L, hs.latencyMs)
        assertEquals(listOf(443), hs.openPorts)
        assertEquals(0, hs.httpStatus) // default when absent
    }

    @Test
    fun `decodes wake result`() {
        val src = """{"device_id":"x","success":true,"mac_address":"AA:BB:CC:DD:EE:FF","sent_to":"192.168.1.255:9"}"""
        val wr = json.decodeFromString<WakeResult>(src)
        assertTrue(wr.success)
        assertEquals("192.168.1.255:9", wr.sentTo)
    }
}
