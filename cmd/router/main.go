package main

import (
	"errors"
	"fmt"
	"github.com/alibaba/kt-connect/pkg/router"
	"github.com/gofrs/flock"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net"
	"os"
	"strings"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

const pathKtLock = "/var/kt.lock"
const actionSetup = "setup"
const actionAdd = "add"
const actionRemove = "remove"
const meshServiceInfix = "-kt-mesh-"

type meshServiceResolver func(string) (bool, error)

func main() {
	os.Exit(run())
}

// run executes the requested action and returns the process exit code.
// A non-zero code is essential: ktctl invokes this binary via ExecInPod and
// only a failing exit code makes routing failures visible to the client side.
func run() int {
	fileLock := flock.New(pathKtLock)
	if err := fileLock.Lock(); err != nil {
		log.Error().Err(err).Msgf("Unable to fetch route lock")
		return 1
	}
	defer fileLock.Unlock()
	if len(os.Args) < 3 {
		usage()
		return 1
	}
	var err error
	switch os.Args[1] {
	case actionSetup:
		err = setup(os.Args[2:])
	case actionAdd:
		err = add(os.Args[2:])
	case actionRemove:
		err = remove(os.Args[2:])
	default:
		log.Error().Msgf("Invalid action '%s'", os.Args[1])
		usage()
		return 1
	}
	if err != nil {
		return 1
	}
	return 0
}

func usage() {
	log.Info().Msgf(`Usage:
router %s <service-name> <service-port> <custom-version>
router %s <custom-version>
router %s <custom-version>
`, actionSetup, actionAdd, actionRemove)
}

func setup(args []string) error {
	if len(args) < 3 {
		usage()
		return fmt.Errorf("setup requires 3 arguments, got %d", len(args))
	}
	header, version := splitVersionMark(args[2])
	ktConf := router.KtConf{
		Service:  args[0],
		Ports:    getPorts(args[1]),
		Header:   header,
		Versions: []string{version},
	}
	if err := router.WriteAndReloadRouteConf(&ktConf); err != nil {
		log.Error().Err(err).Msgf("Write and load route config failed")
		return err
	}
	if err := router.WriteKtConf(&ktConf); err != nil {
		log.Error().Err(err).Msgf("Write kt config failed")
		return err
	}
	log.Info().Msgf("Route setup completed.")
	return nil
}

func add(args []string) error {
	header, version := splitVersionMark(args[0])
	err := updateRoute(header, version, actionAdd)
	if err != nil {
		log.Error().Err(err).Msgf("Update route with add failed")
		return err
	}
	log.Info().Msgf("Route updated.")
	return nil
}

func remove(args []string) error {
	header, version := splitVersionMark(args[0])
	err := updateRoute(header, version, actionRemove)
	if err != nil {
		log.Error().Err(err).Msgf("Update route with remove failed")
		return err
	}
	log.Info().Msgf("Route updated.")
	return nil
}

func splitVersionMark(mark string) (string, string) {
	splits := strings.Split(mark, ":")
	return strings.ReplaceAll(splits[0], "-", "_"), splits[1]
}

func getPorts(portsParameter string) [][]string {
	ports := make([][]string, 0)
	for _, pp := range strings.Split(portsParameter, ",") {
		ports = append(ports, strings.Split(pp, ":"))
	}
	return ports
}

func updateRoute(header, version, action string) error {
	ktConf, err := router.ReadKtConf()
	if err != nil {
		return err
	}
	if ktConf.Header != header {
		return fmt.Errorf("specified header '%s' no match mesh pod header '%s'", header, ktConf.Header)
	}
	keepVersion := ""
	switch action {
	case actionAdd:
		ktConf.Versions = addVersion(ktConf.Versions, version)
		// The mesh service of the version being added may be created just a few
		// seconds ago, so a stale DNS negative cache could hide it. Never prune it.
		keepVersion = version
	case actionRemove:
		ktConf.Versions = removeVersion(ktConf.Versions, version)
	}
	ktConf.Versions = pruneUnavailableVersions(ktConf.Service, ktConf.Versions, keepVersion, resolveMeshService)
	err = router.WriteAndReloadRouteConf(ktConf)
	if err != nil {
		return err
	}
	err = router.WriteKtConf(ktConf)
	if err != nil {
		return err
	}
	return nil
}

func addVersion(versions []string, version string) []string {
	for _, v := range versions {
		if v == version {
			return versions
		}
	}
	return append(versions, version)
}

func removeVersion(versions []string, version string) []string {
	remaining := make([]string, 0, len(versions))
	for _, v := range versions {
		if v != version {
			remaining = append(remaining, v)
		}
	}
	return remaining
}

func pruneUnavailableVersions(service string, versions []string, keepVersion string, resolver meshServiceResolver) []string {
	available := make([]string, 0, len(versions))
	seen := make(map[string]struct{}, len(versions))
	for _, version := range versions {
		if _, ok := seen[version]; ok {
			continue
		}
		seen[version] = struct{}{}

		if version == keepVersion {
			available = append(available, version)
			continue
		}

		meshService := meshServiceName(service, version)
		exists, err := resolver(meshService)
		if err != nil {
			log.Warn().Err(err).Msgf("Unable to resolve mesh service %s, keep version %s", meshService, version)
			available = append(available, version)
			continue
		}
		if !exists {
			log.Warn().Msgf("Prune stale mesh version %s because service %s does not exist", version, meshService)
			continue
		}
		available = append(available, version)
	}
	return available
}

func meshServiceName(service, version string) string {
	return service + meshServiceInfix + version
}

func resolveMeshService(service string) (bool, error) {
	if _, err := net.LookupHost(service); err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
