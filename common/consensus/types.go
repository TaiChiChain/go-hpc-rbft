package consensus

// This file defines all tag related struct used in Trace logger.

const (
	// TagKey is the tag key.
	TagKey = "tag"

	// TagStageKey is the tag stage key.
	TagStageKey = "stage"

	// TagContentKey is the tag content key.
	TagContentKey = "content"
)

const (
	// TagNameViewChange is the name of view change tag.
	TagNameViewChange = "view_change"

	// TagNameEpochChange is the name of epoch change tag.
	TagNameEpochChange = "epoch_change"

	// TagNameCheckpoint is the name of generate checkpoint tag.
	TagNameCheckpoint = "checkpoint"

	// TagNameNotifyCheckpoint is the name of notify checkpoint tag.
	TagNameNotifyCheckpoint = "notify_checkpoint"

	// TagNameSyncChain is the name of sync chain tag.
	TagNameSyncChain = "sync_chain"

	// TagNameNamespaceStart is the name of namespace start tag.
	TagNameNamespaceStart = "ns_start"

	// TagNameNamespaceStop is the name of namespace start tag.
	TagNameNamespaceStop = "ns_stop"

	// TagNameNetworkGRPC is the name of grpc connection tag.
	TagNameNetworkGRPC = "network_grpc"

	// TagNameNetwork is the name of logical connection tag.
	TagNameNetwork = "network"

	// TagNameIPC is the name of ipc tag.
	TagNameIPC = "ipc"
)

const (
	// TagStageStart indicates the start stage of one tag.
	TagStageStart = "start"

	// TagStageFinish indicates the finish stage of one tag.
	TagStageFinish = "finish"

	// TagStageReceive indicates the received stage of one tag.
	TagStageReceive = "receive"

	// TagStageWarning indicates the warning stage of one tag.
	TagStageWarning = "warning"

	// TagStageConnect indicates the successfully connected stage of a connection.
	TagStageConnect = "connect"

	// TagStageDisconnect indicates the successfully disconnected stage of a connection.
	TagStageDisconnect = "disconnect"

	// TagStageConnectFail indicates the process of establishing connection is fail.
	TagStageConnectFail = "connect_fail"
)

// TagValue represents the tag value.
type TagValue struct {
	Tag     string `json:"tag"`
	Stage   string `json:"stage"`
	Content any
}

// TagContentNamespaceModule is the namespace module content.
type TagContentNamespaceModule struct {
	Module string `json:"module"`
}

// TagContentViewChange value.
type TagContentViewChange struct {
	Node uint64 `json:"node"`
	View uint64 `json:"view"`
}

// TagContentEpochChange value.
type TagContentEpochChange struct {
	Epoch        uint64   `json:"epoch"`
	ValidatorSet []string `json:"vset"`
	AlgoVersion  string   `json:"algo_version"`
}

// TagContentCheckpoint value.
type TagContentCheckpoint struct {
	Node   uint64 `json:"node,omitempty"`
	Height uint64 `json:"height"`

	// false: normal
	// true: config
	Config bool `json:"config"`
}

// TagContentSyncChain value.
type TagContentSyncChain struct {
	From        uint64 `json:"from"`
	To          uint64 `json:"to"`
	TargetEpoch uint64 `json:"target_epoch,omitempty"`
}

// TagContentInconsistentCheckpoint value, with same height but different checkpoint state.
type TagContentInconsistentCheckpoint struct {
	// same height
	Height uint64 `json:"height"`

	// different checkpoint state.
	// key: CommonCheckpointState, value: node list.
	CheckpointSet map[CommonCheckpointState][]uint64 `json:"checkpoint_set"`
}

// CommonCheckpointState value.
type CommonCheckpointState struct {
	// checkpoint epoch.
	Epoch uint64 `json:"uint64"`

	// checkpoint block hash.
	Hash string `json:"hash"`
}

// TagContentConnect value.
type TagContentConnect struct {
	Hostname string `json:"hostname"` // the hostname of remote peer
}

// TagContentDisconnect value.
type TagContentDisconnect struct {
	Hostname string `json:"hostname"`         // the hostname of remote peer
	Reason   string `json:"reason,omitempty"` // the reason of disconnecting
}

// TagContentIPC value.
type TagContentIPC struct {
	Command string `json:"command"`         // ip command
	Error   string `json:"error,omitempty"` // optionally processing error result
}

type RbftQuorumCheckpoint struct {
	QuorumCheckpoint
}

func (q *RbftQuorumCheckpoint) Epoch() uint64 {
	return q.QuorumCheckpoint.Epoch()
}

func (q *RbftQuorumCheckpoint) NextEpoch() uint64 {
	return q.QuorumCheckpoint.NextEpoch()
}

func (q *RbftQuorumCheckpoint) Marshal() ([]byte, error) {
	return q.MarshalVT()
}

func (q *RbftQuorumCheckpoint) Unmarshal(raw []byte) error {
	return q.UnmarshalVT(raw)
}

func (q *RbftQuorumCheckpoint) GetStateDigest() string {
	return q.Digest()
}

func (q *RbftQuorumCheckpoint) GetHeight() uint64 {
	return q.Height()
}
