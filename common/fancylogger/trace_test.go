package fancylogger

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/hyperchain/go-hpc-rbft/v2/common/consensus"
)

func TestTrace(t *testing.T) {
	csLogger := NewLogger("consensus", INFO)
	_ = csLogger.WithField("namespace", "global")
	backend := &multiBackends{[]Backend{
		NewIOBackend(&StringFormatter{true, false,
			"2006-01-02T15:04:05.000", false, false,
			true, true, FieldMap{}}, os.Stdout),
		NewIOBackend(&JSONFormatter{"2006-01-02T15:04:05.000", false, FieldMap{}, false}, os.Stdout)}}

	//backend := &multiBackends{[]Backend{
	//	NewIOBackend(&StringFormatter{true, false,
	//		"2006-01-02T15:04:05.000", false, false,
	//		true, true, FieldMap{}}, os.Stdout)}}

	//backend := &multiBackends{[]Backend{
	//	NewIOBackend(&JSONFormatter{"2006-01-02T15:04:05.000", false, FieldMap{}, false}, os.Stdout)}}

	csLogger.SetBackends(backend)
	csLogger.SetTraceBackends(backend)

	// start epoch 1.

	csLogger.Trace(consensus.TagNameEpochChange, consensus.TagStageFinish, consensus.TagContentEpochChange{
		Epoch:        1,
		ValidatorSet: []string{"node1", "node2", "node3", "node4"},
		AlgoVersion:  "RBFT@1.0",
	})

	csLogger.Trace(consensus.TagNameViewChange, consensus.TagStageStart, consensus.TagContentViewChange{
		Node: 1,
		View: 1,
	})

	csLogger.Trace(consensus.TagNameViewChange, consensus.TagStageReceive, consensus.TagContentViewChange{
		Node: 2,
		View: 1,
	})

	csLogger.Trace(consensus.TagNameViewChange, consensus.TagStageReceive, consensus.TagContentViewChange{
		Node: 3,
		View: 1,
	})

	csLogger.Trace(consensus.TagNameViewChange, consensus.TagStageFinish, consensus.TagContentViewChange{
		Node: 1,
		View: 1,
	})

	csLogger.Trace(consensus.TagNameCheckpoint, consensus.TagStageStart, consensus.TagContentCheckpoint{
		Node:   "node1",
		Height: 10,
		Config: false,
	})

	csLogger.Trace(consensus.TagNameCheckpoint, consensus.TagStageReceive, consensus.TagContentCheckpoint{
		Node:   "node2",
		Height: 10,
		Config: false,
	})

	csLogger.Trace(consensus.TagNameCheckpoint, consensus.TagStageReceive, consensus.TagContentCheckpoint{
		Node:   "node4",
		Height: 10,
		Config: false,
	})

	csLogger.Trace(consensus.TagNameCheckpoint, consensus.TagStageFinish, consensus.TagContentCheckpoint{
		Node:   "node1",
		Height: 10,
		Config: false,
	})

	th := NewTagHook()
	th.AddHook(consensus.TagStageWarning, "127.0.0.1:8001")
	csLogger.SetHooks(th)
	csLogger.Trace(consensus.TagNameCheckpoint, consensus.TagStageWarning, consensus.TagContentInconsistentCheckpoint{
		Height: 10,
		CheckpointSet: map[consensus.CommonCheckpointState][]string{
			{Epoch: 1, Hash: "hash-1"}: {"node1", "node2"},
			{Epoch: 1, Hash: "hash-2"}: {"node3", "node4"},
		},
	})

	// parse json format output.
	sv := `
{"content":{"epoch":1,"vset":["node1","node2","node3","node4"],"algo_version":"RBFT@1.0"},"module":"consensus","namespace":"global","stage":"finish","tag":"epoch_change","time":"2023-05-05T16:08:05.968"}
`
	var output interface{}
	_ = json.Unmarshal([]byte(sv), &output)
	t.Log(output)
	outerMap := output.(map[string]interface{})
	for k, v := range outerMap {
		t.Logf("outer key: %s\n", k)
		t.Logf("outer value: %s\n", v)
		if k == "tag" && v == consensus.TagNameEpochChange {
			t.Log("find epoch change")
			v2 := outerMap["content"].(map[string]interface{})
			t.Log("new epoch info:")
			for kk, vv := range v2 {
				t.Logf("inner key: %s\n", kk)
				t.Logf("inner value: %s\n", vv)
			}
		}
	}
}
