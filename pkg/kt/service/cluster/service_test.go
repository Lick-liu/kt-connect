package cluster

import (
	opt "github.com/alibaba/kt-connect/pkg/kt/command/options"
	"github.com/alibaba/kt-connect/pkg/kt/util"
	testclient "k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestKubernetes_CreateService(t *testing.T) {
	type args struct {
		name        string
		namespace   string
		port        map[int]int
		labels      map[string]string
		annotations map[string]string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "shouldCreateService",
			args: args{
				name:      "svc-name",
				namespace: "default",
				port:      map[int]int{8080: 8080},
				labels: map[string]string{
					"label": "value",
				},
				annotations: map[string]string{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &Kubernetes{
				Clientset: testclient.NewSimpleClientset(),
			}
			_, err := k.CreateService(&SvcMetaAndSpec{
				Meta: &ResourceMeta{
					Name:        tt.args.name,
					Namespace:   tt.args.namespace,
					Labels:      map[string]string{util.ControlBy: util.KubernetesToolkit},
					Annotations: tt.args.annotations,
				},
				External:  false,
				Ports:     tt.args.port,
				Selectors: tt.args.labels,
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("Kubernetes.CreateService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestCreateServiceAddsRuntimeAnnotations(t *testing.T) {
	oldSessionID := opt.Store.SessionID
	oldComponent := opt.Store.Component
	oldMesh := opt.Store.Mesh
	opt.Store.SessionID = "session-1"
	opt.Store.Component = util.ComponentMesh
	opt.Store.Mesh = "zodance-version:jz2"
	defer func() {
		opt.Store.SessionID = oldSessionID
		opt.Store.Component = oldComponent
		opt.Store.Mesh = oldMesh
	}()

	svc := createService(&SvcMetaAndSpec{
		Meta: &ResourceMeta{
			Name:        "svc-name",
			Namespace:   "default",
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		External:  false,
		Ports:     map[int]int{8080: 8080},
		Selectors: map[string]string{"app": "demo"},
	})

	if got := svc.Annotations[util.KtSessionID]; got != "session-1" {
		t.Fatalf("session annotation = %q, want session-1", got)
	}
	if got := svc.Annotations[util.KtComponent]; got != util.ComponentMesh {
		t.Fatalf("component annotation = %q, want %s", got, util.ComponentMesh)
	}
	if got := svc.Annotations[util.KtVersionMark]; got != "zodance-version:jz2" {
		t.Fatalf("version annotation = %q, want zodance-version:jz2", got)
	}
}
