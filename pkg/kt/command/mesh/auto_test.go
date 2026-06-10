package mesh

import (
	"strconv"
	"testing"

	"github.com/alibaba/kt-connect/pkg/kt/util"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsStaleMeshShadow(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{
			name: "fresh heartbeat is active",
			annotations: map[string]string{
				util.KtLastHeartBeat: util.GetTimestamp(),
			},
			want: false,
		},
		{
			name: "expired heartbeat is stale",
			annotations: map[string]string{
				util.KtLastHeartBeat: oldHeartbeat(),
			},
			want: true,
		},
		{
			name:        "missing heartbeat is not auto deleted",
			annotations: map[string]string{},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &coreV1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}
			if got := isStaleMeshShadow(pod); got != tt.want {
				t.Fatalf("isStaleMeshShadow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func oldHeartbeat() string {
	return formatHeartbeat(util.GetTime() - (staleMeshShadowThresholdMinutes+1)*60)
}

func formatHeartbeat(timestamp int64) string {
	return strconv.FormatInt(timestamp, 10)
}
