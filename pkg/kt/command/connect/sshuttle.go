package connect

import (
	"github.com/alibaba/kt-connect/pkg/common"
	opt "github.com/alibaba/kt-connect/pkg/kt/command/options"
	"github.com/alibaba/kt-connect/pkg/kt/service/cluster"
	"github.com/alibaba/kt-connect/pkg/kt/service/sshuttle"
	"github.com/alibaba/kt-connect/pkg/kt/transmission"
	"github.com/alibaba/kt-connect/pkg/kt/util"
	"github.com/rs/zerolog/log"
	"time"
)

func BySshuttle() error {
	checkSshuttleInstalled()

	podIP, podName, privateKeyPath, err := getOrCreateShadow()
	if err != nil {
		return err
	}

	cidr, excludeCidr := cluster.Ins().ClusterCidr(opt.Get().Global.Namespace)

	localSshPort := util.GetRandomTcpPort()
	if _, err = transmission.SetupPortForwardToLocal(podName, common.StandardSshPort, localSshPort); err != nil {
		return err
	}

	req := &sshuttle.SSHVPNRequest{
		LocalSshPort:           localSshPort,
		RemoteSSHPKPath:        privateKeyPath,
		RemoteDNSServerAddress: podIP,
		IncludeCIDR:            cidr,
		ExcludeCIDR:            excludeCidr,
	}
	if err = startSshuttle(req); err != nil {
		return err
	}

	return setupDns(podName, podIP)
}

func startSshuttle(req *sshuttle.SSHVPNRequest) error {
	return startSshuttleWithFailures(req, 0)
}

func startSshuttleWithFailures(req *sshuttle.SSHVPNRequest, failures int) error {
	res := make(chan error)
	if err := util.BackgroundRun(sshuttle.Ins().Connect(req), "vpn(sshuttle)", res); err != nil {
		return err
	}

	started := time.Now()
	go func() {
		select {
		case <-res:
			nextFailures := failures + 1
			if time.Since(started) >= time.Second {
				nextFailures = 0
			}
			if transmission.ExitAfterRepeatedReconnectFailures("sshuttle", nextFailures) {
				return
			}
			time.Sleep(10 * time.Second)
			log.Debug().Msgf("Restarting sshuttle ...")
			_ = startSshuttleWithFailures(req, nextFailures)
		}
	}()

	return nil
}

func checkSshuttleInstalled() {
	if !util.CanRun(sshuttle.Ins().Version()) {
		_, _, err := util.RunAndWait(sshuttle.Ins().Install())
		if err != nil {
			log.Error().Err(err).Msgf("Failed find or install sshuttle")
		}
	}
}
