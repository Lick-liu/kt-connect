package transmission

import (
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
)

func Test_parseReqHost(t *testing.T) {
	type args struct {
		host string
		path string
	}
	tests := []struct {
		name string
		args args
		want *url.URL
	}{
		{
			name: "without port",
			args: args{
				host: "http://1.2.3.4",
				path: "/api/v1/test",
			},
			want: &url.URL{Scheme: "http", Host: "1.2.3.4", Path: "/api/v1/test"},
		},
		{
			name: "with port",
			args: args{
				host: "https://1.2.3.4:6443",
				path: "/api/v1/test",
			},
			want: &url.URL{Scheme: "https", Host: "1.2.3.4:6443", Path: "/api/v1/test"},
		},
		{
			name: "with extra url",
			args: args{
				host: "https://1.2.3.4:6443/k8s/cluster",
				path: "/api/v1/test",
			},
			want: &url.URL{Scheme: "https", Host: "1.2.3.4:6443", Path: "/k8s/cluster/api/v1/test"},
		},
		{
			name: "with extra url and slash ending",
			args: args{
				host: "https://1.2.3.4:6443/k8s/cluster/",
				path: "/api/v1/test",
			},
			want: &url.URL{Scheme: "https", Host: "1.2.3.4:6443", Path: "/k8s/cluster/api/v1/test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := parseReqHost(tt.args.host, tt.args.path); !reflect.DeepEqual(*got, *tt.want) {
				t.Errorf("parseReqHost() = %v, want %v", *got, *tt.want)
			}
		})
	}
}

func TestPortForwardReconnectUsesFailureThreshold(t *testing.T) {
	content, err := os.ReadFile("portforward.go")
	if err != nil {
		t.Fatalf("read portforward.go: %v", err)
	}
	if !strings.Contains(string(content), `ExitAfterRepeatedReconnectFailures("Port forward"`) {
		t.Fatalf("port-forward reconnect path should exit after repeated failures")
	}
}

func TestSshuttleReconnectUsesFailureThreshold(t *testing.T) {
	content, err := os.ReadFile("../command/connect/sshuttle.go")
	if err != nil {
		t.Fatalf("read sshuttle.go: %v", err)
	}
	if !strings.Contains(string(content), `ExitAfterRepeatedReconnectFailures("sshuttle"`) {
		t.Fatalf("sshuttle reconnect path should exit after repeated failures")
	}
}
