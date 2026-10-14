package worker_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/concourse/concourse/v8/atc"
	"github.com/concourse/concourse/v8/integration/internal/dctest"
	"github.com/concourse/concourse/v8/integration/internal/flytest"
	"github.com/concourse/concourse/v8/integration/internal/ypath"
	"github.com/stretchr/testify/require"
)

func TestP2PStreamingGroups(t *testing.T) {
	cases := []struct {
		name, sourceGroup, destinationGroup string
		enabled, wantP2P                    bool
	}{
		{"same-group", "group-a", "group-a", true, true},
		{"different-groups", "group-a", "group-b", true, false},
		{"ungrouped", "", "", true, true},
		{"grouped-source", "group-a", "", true, false},
		{"grouped-destination", "", "group-a", true, false},
		{"disabled", "group-a", "group-a", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := ypath.Load(t, "../docker-compose.yml")
			doc.Set(t, "$.services.web.environment.CONCOURSE_ENABLE_P2P_VOLUME_STREAMING", strconv.FormatBool(tc.enabled))
			doc.Move(t, "$.services.worker", "$.services.worker1")
			doc.Clone(t, "$.services.worker1", "$.services.worker2")
			for _, w := range []struct{ name, tag, group string }{
				{"worker1", "source", tc.sourceGroup},
				{"worker2", "destination", tc.destinationGroup},
			} {
				prefix := "$.services." + w.name + ".environment."
				doc.Set(t, prefix+"CONCOURSE_NAME", w.name)
				doc.Set(t, prefix+"CONCOURSE_TAG", w.tag)
				// Advertise an address the other worker can actually reach. The web
				// node continues to reach baggageclaim via the TSA forwarding tunnel.
				doc.Set(t, prefix+"CONCOURSE_BAGGAGECLAIM_BIND_IP", "0.0.0.0")
				doc.Set(t, prefix+"CONCOURSE_BAGGAGECLAIM_P2P_INTERFACE_NAME_PATTERN", "eth0")
				if w.group != "" {
					doc.Set(t, prefix+"CONCOURSE_P2P_STREAMING_GROUP", w.group)
				}
			}
			dc := dctest.InitDynamic(t, doc, "..")
			dc.Run(t, "up", "-d")
			fly := flytest.Init(t, dc)
			require.Eventually(t, func() bool {
				var workers []atc.Worker
				fly.Silence().OutputJSON(t, &workers, "curl", "/api/v1/workers", "--", "--silent", "--fail")
				return len(workers) == 2 && workers[0].State == "running" && workers[1].State == "running"
			}, time.Minute, time.Second)
			var workers []atc.Worker
			fly.OutputJSON(t, &workers, "curl", "/api/v1/workers", "--", "--silent", "--fail")
			groups := map[string]string{}
			for _, w := range workers {
				groups[w.Name] = w.P2PStreamingGroup
			}
			require.Equal(t, map[string]string{"worker1": tc.sourceGroup, "worker2": tc.destinationGroup}, groups)

			fly.Run(t, "set-pipeline", "-n", "-c", "pipelines/p2p_streaming_groups.yml", "-p", "streaming")
			fly.Run(t, "unpause-pipeline", "-p", "streaming")
			output := fly.Output(t, "trigger-job", "-j", "streaming/transfer", "--watch")
			require.Contains(t, output, "p2p-streaming-groups-ok")
			require.Contains(t, output, "blob: OK")

			sourceLogs := dc.Output(t, "logs", "--no-color", "worker1")
			webLogs := dc.Output(t, "logs", "--no-color", "web")
			require.NotContains(t, webLogs, "p2p-stream-failed-falling-back-to-atc")
			if tc.wantP2P {
				require.Contains(t, sourceLogs, "stream-p2p-out.done")
			} else {
				require.NotContains(t, sourceLogs, "stream-p2p-out")
				require.Contains(t, sourceLogs, "stream-out.done")
			}
		})
	}
}
