package router

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"
)

//go:embed route.conf
var routeTemplate string

const pathRouteConf = "/etc/nginx/conf.d/route.conf"

func WriteAndReloadRouteConf(ktConf *KtConf) error {
	var err error
	if len(ktConf.Versions) > 0 {
		err = writeRouteConf(ktConf)
	} else {
		err = removeRouteConf()
	}
	if err != nil {
		return err
	}
	err = reloadRouteConf()
	if err != nil {
		return err
	}
	return nil
}

func reloadRouteConf() error {
	process, err := os.FindProcess(1)
	if err != nil {
		return fmt.Errorf("failed to find route process: %s", err)
	}
	err = process.Signal(syscall.SIGHUP)
	if err != nil {
		return fmt.Errorf("failed to reload route configuration: %s", err)
	}
	return nil
}

func writeRouteConf(ktConf *KtConf) error {
	content, err := renderRouteConf(ktConf)
	if err != nil {
		return err
	}
	return replaceRouteConfWithValidation(pathRouteConf, content, validateRouteConf)
}

func renderRouteConf(ktConf *KtConf) ([]byte, error) {
	tmpl, err := template.New("route").Parse(routeTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to load route template: %s", err)
	}

	var routeConf bytes.Buffer
	err = tmpl.Execute(&routeConf, ktConf)
	if err != nil {
		return nil, fmt.Errorf("failed to generate route configuration: %s", err)
	}
	return routeConf.Bytes(), nil
}

func validateRouteConf() error {
	out, err := exec.Command("nginx", "-t").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func removeRouteConf() error {
	return replaceRouteConfWithValidation(pathRouteConf, nil, validateRouteConf)
}

func replaceRouteConfWithValidation(path string, content []byte, validate func() error) error {
	previous, err := os.ReadFile(path)
	previousExists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing route configuration: %s", err)
	}

	if err := replaceRouteConf(path, content); err != nil {
		return err
	}
	if validate != nil {
		if err := validate(); err != nil {
			if restoreErr := restoreRouteConf(path, previous, previousExists); restoreErr != nil {
				return fmt.Errorf("route configuration validation failed: %s; restore failed: %s", err, restoreErr)
			}
			return fmt.Errorf("route configuration validation failed: %s", err)
		}
	}
	return nil
}

func replaceRouteConf(path string, content []byte) error {
	if content == nil {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove route configuration: %s", err)
		}
		return nil
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".route-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary route configuration: %s", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err = tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write temporary route configuration: %s", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temporary route configuration: %s", err)
	}
	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to replace route configuration: %s", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to install route configuration: %s", err)
	}
	return nil
}

func restoreRouteConf(path string, content []byte, exists bool) error {
	if !exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return os.WriteFile(path, content, 0644)
}
