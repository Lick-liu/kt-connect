package sshchannel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/alibaba/kt-connect/pkg/kt/util"
	"io"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wzshiming/socks5"
	"github.com/wzshiming/sshproxy"
)

type SocksLogger struct{}

const localDialTimeout = 500 * time.Millisecond

type localDialer func(network, address string, timeout time.Duration) (net.Conn, error)

func (s SocksLogger) Println(v ...any) {
	_, _ = util.BackgroundLogger.Write([]byte(fmt.Sprint(v...) + util.Eol))
}

// StartSocks5Proxy start socks5 proxy
func (c *Cli) StartSocks5Proxy(privateKey, sshAddress, socks5Address string) (err error) {
	dialer, err := sshproxy.NewDialer(getSshTunnelAddress(privateKey, sshAddress))
	if err != nil {
		return err
	}
	defer dialer.Close()

	svc := &socks5.Server{
		Logger:    SocksLogger{},
		ProxyDial: dialer.DialContext,
	}
	return svc.ListenAndServe("tcp", socks5Address)
}

// RunScript run the script on remote host.
func (c *Cli) RunScript(privateKey, sshAddress, script string) (result string, err error) {
	dialer, err := sshproxy.NewDialer(getSshTunnelAddress(privateKey, sshAddress))
	if err != nil {
		return "", err
	}
	defer dialer.Close()

	conn, err := dialer.SSHClient(context.Background())
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create ssh tunnel")
		return "", err
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create ssh session")
		return "", err
	}
	defer session.Close()

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	err = session.Run(script)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to run ssh script")
		return "", err
	}
	output := stdoutBuf.String()
	return output, nil
}

// ForwardRemoteToLocal forward remote request to local
func (c *Cli) ForwardRemoteToLocal(privateKey, sshAddress, remoteEndpoint, localEndpoint string) error {
	// Handle incoming connections on reverse forwarded tunnel
	dialer, err := sshproxy.NewDialer(getSshTunnelAddress(privateKey, sshAddress))
	if err != nil {
		return err
	}
	defer dialer.Close()

	_, err = dialer.SSHClient(context.Background())
	if err != nil {
		log.Debug().Err(err).Msgf("Failed to create ssh tunnel")
		return err
	}

	// Listen on remote server port of shadow pod, via ssh connection
	listener, err := dialer.Listen(context.Background(), "tcp", remoteEndpoint)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to listen remote endpoint")
		disconnectRemotePort(privateKey, sshAddress, remoteEndpoint, c)
		return err
	}
	defer listener.Close()

	log.Info().Msgf("Reverse tunnel %s -> %s established", remoteEndpoint, localEndpoint)
	for {
		if err = handleRequest(listener, localEndpoint); errors.Is(err, io.EOF) {
			return err
		}
	}
}

func getSshTunnelAddress(privateKey string, sshAddress string) string {
	return fmt.Sprintf("ssh://root@%s?identity_file=%s", sshAddress, privateKey)
}

func disconnectRemotePort(privateKey, sshAddress, remoteEndpoint string, c *Cli) {
	remotePort := strings.Split(remoteEndpoint, ":")[1]
	out, err := c.RunScript(privateKey, sshAddress, fmt.Sprintf("/disconnect.sh %s", remotePort))
	if out != "" {
		_, _ = util.BackgroundLogger.Write([]byte(out + util.Eol))
	}
	if err != nil {
		log.Warn().Err(err).Msgf("Failed to disconnect remote port %s", remotePort)
	}
}

func handleRequest(listener net.Listener, localEndpoint string) error {
	return handleRequestWithDial(listener, localEndpoint, net.DialTimeout)
}

func handleRequestWithDial(listener net.Listener, localEndpoint string, dial localDialer) error {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Msgf("Failed to handle request: %v", r)
		}
	}()

	// Wait requests from remote endpoint
	client, err := listener.Accept()
	if err != nil {
		log.Error().Err(err).Msgf("Failed to accept remote request")
		if !errors.Is(err, io.EOF) {
			time.Sleep(2 * time.Second)
		}
		return err
	}

	// Never connect to the local service in the accept loop. A stopped or
	// black-holed local port can make Dial block for seconds; doing that here
	// serializes every remote request behind the previous failure and produces
	// staircase latency under concurrent load.
	go connectLocalAndHandle(client, localEndpoint, dial)
	return nil
}

func connectLocalAndHandle(client net.Conn, localEndpoint string, dial localDialer) {
	// Open a (local) connection to localEndpoint whose content will be forwarded to remoteEndpoint
	local, err := dial("tcp", localEndpoint, localDialTimeout)
	if err != nil {
		_ = client.Close()
		log.Error().Err(err).Msgf("Local service error")
		return
	}

	handleClient(client, local)
}

func handleClient(client net.Conn, remote net.Conn) {
	done := make(chan int)

	// Start remote -> local data transfer
	remoteReader := util.NewInterpretableReader(remote)
	go func() {
		defer handleBrokenTunnel(done)
		if _, err := io.Copy(client, remoteReader); err != nil {
			logCopyError(err, "remote->local")
		}
		done <- 1
	}()

	// Start local -> remote data transfer
	localReader := util.NewInterpretableReader(client)
	go func() {
		defer handleBrokenTunnel(done)
		if _, err := io.Copy(remote, localReader); err != nil {
			logCopyError(err, "local->remote")
		}
		done <- 1
	}()

	<-done
	remoteReader.Cancel()
	localReader.Cancel()
	_ = remote.Close()
	_ = client.Close()
}

func logCopyError(err error, direction string) {
	if shouldIgnoreCopyError(err) {
		log.Debug().Err(err).Msgf("Ignore closed tunnel copy %s", direction)
		return
	}

	log.Warn().Err(err).Msgf("Error while copy %s", direction)
}

func shouldIgnoreCopyError(err error) bool {
	if err == nil {
		return false
	}

	return errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "use of closed network connection")
}

func handleBrokenTunnel(done chan int) {
	if r := recover(); r != nil {
		log.Error().Msgf("Ssh tunnel broken: %v", r)
		done <- 1
	}
}
