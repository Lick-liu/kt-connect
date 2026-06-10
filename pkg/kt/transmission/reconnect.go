package transmission

import (
	"os"
	"time"

	"github.com/rs/zerolog/log"
)

const maxTunnelReconnectFailures = 3
const reconnectStableDuration = time.Second

var exitProcess = os.Exit

func shouldExitAfterReconnectFailure(failures int) bool {
	return failures >= maxTunnelReconnectFailures
}

func nextReconnectFailures(currentFailures int, stable bool) int {
	if stable {
		return 0
	}
	return currentFailures + 1
}

func ExitAfterRepeatedReconnectFailures(tunnelName string, failures int) bool {
	if !shouldExitAfterReconnectFailure(failures) {
		return false
	}
	log.Error().Msgf("%s failed to reconnect %d times, exiting for supervisor restart", tunnelName, failures)
	exitProcess(1)
	return true
}
