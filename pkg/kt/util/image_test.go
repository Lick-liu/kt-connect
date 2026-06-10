package util

import "testing"

func TestDefaultKtImagesUseChengduRegistry(t *testing.T) {
	if ImageKtShadow != "registry.cn-chengdu.aliyuncs.com/zodancer/kt-connect-shadow" {
		t.Fatalf("ImageKtShadow = %q", ImageKtShadow)
	}
	if ImageKtRouter != "registry.cn-chengdu.aliyuncs.com/zodancer/kt-connect-router" {
		t.Fatalf("ImageKtRouter = %q", ImageKtRouter)
	}
}
