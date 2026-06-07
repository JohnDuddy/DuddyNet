package net.duddy.duddynet

import net.duddy.duddynet.data.ApiClientFactory
import org.junit.Assert.assertEquals
import org.junit.Test

class BaseUrlTest {
    @Test
    fun `magicdns name gets default port and scheme`() {
        assertEquals("http://duddynet-agent.tailXXXX.ts.net:8765/", ApiClientFactory.baseUrl("duddynet-agent.tailXXXX.ts.net"))
    }

    @Test
    fun `tailscale ip gets default port`() {
        assertEquals("http://100.1.2.3:8765/", ApiClientFactory.baseUrl("100.1.2.3"))
    }

    @Test
    fun `explicit port is preserved`() {
        assertEquals("http://100.1.2.3:9000/", ApiClientFactory.baseUrl("100.1.2.3:9000"))
    }

    @Test
    fun `full url is respected`() {
        assertEquals("https://nas.example:443/", ApiClientFactory.baseUrl("https://nas.example:443"))
    }

    @Test
    fun `blank host yields empty`() {
        assertEquals("", ApiClientFactory.baseUrl("  "))
    }
}
