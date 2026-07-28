package util

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractHostIp(t *testing.T) {
	require.Equal(t, "1.2.3.4", ExtractHostIp("http://1.2.3.4"))
	require.Equal(t, "1.2.3.4", ExtractHostIp("http://1.2.3.4:8080"))
	require.Equal(t, "1.2.3.4", ExtractHostIp("http://1.2.3.4:8080/a/b/c"))
	require.Equal(t, "127.0.0.1", ExtractHostIp("http://localhost:8080/a/b/c"))
}

func TestGetRandomTcpPortReturnsListenablePort(t *testing.T) {
	for i := 0; i < 5; i++ {
		port := GetRandomTcpPort()
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		require.NoError(t, err, "port %d returned by GetRandomTcpPort must be listenable", port)
		_ = listener.Close()
	}
}
