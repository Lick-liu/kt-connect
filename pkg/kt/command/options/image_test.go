package options

import (
	"fmt"
	"testing"

	"github.com/alibaba/kt-connect/pkg/kt/util"
)

func TestClientVersionDoesNotSelectUnpublishedRuntimeImages(t *testing.T) {
	previousVersion := Store.Version
	Store.Version = "1.3-dev"
	defer func() { Store.Version = previousVersion }()

	wantShadow := fmt.Sprintf("%s:%s", util.ImageKtShadow, util.DefaultKtRuntimeImageTag)
	wantRouter := fmt.Sprintf("%s:%s", util.ImageKtRouter, util.DefaultKtRuntimeImageTag)

	if got := optionDefault(GlobalFlags(), "Image"); got != wantShadow {
		t.Fatalf("shadow image default = %v, want %q", got, wantShadow)
	}
	if got := optionDefault(MeshFlags(), "RouterImage"); got != wantRouter {
		t.Fatalf("router image default = %v, want %q", got, wantRouter)
	}
}

func optionDefault(configs []OptionConfig, target string) any {
	for _, config := range configs {
		if config.Target == target {
			return config.DefaultValue
		}
	}
	return nil
}
