package rbft

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"

	"github.com/axiomesh/axiom-ledger/pkg/txpool"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/common/metrics/disabled"
	"github.com/axiomesh/axiom-bft/types"
)

var peerSet = []NodeInfo{
	{
		ID:                   1,
		AccountAddress:       "node1",
		P2PNodeID:            "node1",
		ConsensusVotingPower: 1000,
	},
	{
		ID:                   2,
		AccountAddress:       "node2",
		P2PNodeID:            "node2",
		ConsensusVotingPower: 1000,
	},
	{
		ID:                   3,
		AccountAddress:       "node3",
		P2PNodeID:            "node3",
		ConsensusVotingPower: 1000,
	},
	{
		ID:                   4,
		AccountAddress:       "node4",
		P2PNodeID:            "node4",
		ConsensusVotingPower: 1000,
	},
}

// testFramework contains the core structure of test framework instance.
type testFramework[T any, Constraint consensus.TXConstraint[T]] struct {
	N int

	// Instance of nodes.
	TestNode []*testNode[T, Constraint]

	// testFramework.Peers is the router map in cluster.
	// we could regard it as the routerInfo of epoch in trusted node
	Router []*NodeInfo

	// Channel to close this event process.
	close chan bool

	// Channel to receive messages sent from nodes in cluster.
	clusterChan chan *channelMsg

	// delFlag
	delFlag chan bool

	// Write logger to record some info.
	log common.Logger
}

// testNode contains the parameters of one node instance.
type testNode[T any, Constraint consensus.TXConstraint[T]] struct {
	Logger common.Logger

	// Node is provided for application to contact wih RBFT core.
	N Node[T, Constraint]

	// n is provided for testers to check RBFT core info.
	// Generally not used.
	n *node[T, Constraint]

	// testNode.Peers is the router map in one node.
	Router []NodeInfo

	// epoch info in mem-chain
	Epoch uint64

	// last config transaction in mem-chain
	Applied uint64

	Digest string

	// block storage
	blocks map[uint64]string

	// ID is the num-identity of the local rbft.peerMgr.localIDde.
	ID uint64

	P2PNodeID string

	// Normal indicates if current node could deal with messages.
	normal bool

	// Online indicates if current node could receive or send messages.
	online bool

	// Channel to close this event process.
	close chan bool

	// Channel to receive messages in cluster.
	recvChan chan *consensusMessageWrapper

	// Storage of consensus logs.
	stateStore map[string][]byte

	// consensus message cache for broadcast
	broadcastMessageCache *consensusMessageWrapper

	// consensus message cache for unicast
	unicastMessageCache *consensusMessageWrapper
}

// testExternal is the instance of External interface.
type testExternal[T any, Constraint consensus.TXConstraint[T]] struct {
	tf *testFramework[T, Constraint]

	// testNode indicates which node the Service belongs to.
	testNode *testNode[T, Constraint]

	// Channel to receive messages sent from nodes in cluster.
	clusterChan chan *channelMsg

	// mem chain for framework
	lastConfigCheckpoint *consensus.QuorumCheckpoint

	// config checkpoint record
	configCheckpointRecord map[uint64]*consensus.EpochChange
}

// channelMsg is the form of data in cluster network.
type channelMsg struct {
	// target node of consensus message.
	// if it is "", the channelMsg is a broadcast message.
	// else it means the target node to unicast message.
	to string

	// consensus message
	msg *consensus.ConsensusMessage
}

// =============================================================================
// init process
// =============================================================================
// newTestFramework init the testFramework instance
func newTestFramework[T any, Constraint consensus.TXConstraint[T]](account int) *testFramework[T, Constraint] {
	// Init PeerSet
	var routers []*NodeInfo
	for i := 0; i < account; i++ {
		id := uint64(i + 1)
		n := "node" + strconv.Itoa(i+1)
		peer := &NodeInfo{
			ID:                   id,
			AccountAddress:       n,
			P2PNodeID:            n,
			ConsensusVotingPower: 1,
		}
		routers = append(routers, peer)
	}

	cc := make(chan *channelMsg, 1)
	// Init Framework
	delFlag := make(chan bool)
	tf := &testFramework[T, Constraint]{
		TestNode: nil,
		Router:   routers,

		close:       make(chan bool),
		clusterChan: cc,

		delFlag: delFlag,

		log: common.NewSimpleLogger(),
	}

	tf.log.Debugf("routers:")
	for _, peer := range tf.Router {
		tf.log.Debugf("ID: %d", peer.ID)
	}

	// set node number
	tf.setN(len(tf.Router))

	// Init testNode in TestFramework
	for i := range tf.Router {
		tn := tf.newTestNode(tf.Router[i].ID, tf.Router[i].AccountAddress, cc)
		tf.TestNode = append(tf.TestNode, tn)
	}

	return tf
}

// newNodeConfig init the Config of Node.
func (tf *testFramework[T, Constraint]) newNodeConfig(
	p2pNodeID string,
	log common.Logger,
	epoch uint64) Config {
	return Config{
		GenesisEpochInfo: &EpochInfo{
			Version:                   1,
			Epoch:                     epoch,
			EpochPeriod:               1000,
			CandidateSet:              []NodeInfo{},
			ValidatorSet:              peerSet,
			StartBlock:                0,
			P2PBootstrapNodeAddresses: []string{"1"},
			ConsensusParams: ConsensusParams{
				ValidatorElectionType:         ValidatorElectionTypeWRF,
				ProposerElectionType:          ProposerElectionTypeAbnormalRotation,
				CheckpointPeriod:              10,
				HighWatermarkCheckpointPeriod: 4,
				MaxValidatorNum:               10,
				BlockMaxTxNum:                 100,
				NotActiveWeight:               1,
				AbnormalNodeExcludeView:       10,
			},
			FinanceParams: FinanceParams{
				GasLimit:              0x5f5e100,
				MaxGasPrice:           10000000000000,
				MinGasPrice:           1000000000000,
				GasChangeRateValue:    1250,
				GasChangeRateDecimals: 4,
			},
			MiscParams: MiscParams{
				TxMaxSize: 10 * 32 * 1024,
			},
		},
		SelfAccountAddress: p2pNodeID,
		SetSize:            25,
		LastServiceState: &types.ServiceState{
			MetaState: &types.MetaState{},
			Epoch:     epoch,
		},
		BatchTimeout:            2 * time.Second, // make sure the batch is too large to not trigger generate batch timer
		RequestTimeout:          6 * time.Second,
		NullRequestTimeout:      9 * time.Second,
		VcResendTimeout:         10 * time.Second,
		CleanVCTimeout:          60 * time.Second,
		NewViewTimeout:          8 * time.Second,
		SyncStateTimeout:        1 * time.Second,
		SyncStateRestartTimeout: 10 * time.Second,
		FetchCheckpointTimeout:  5 * time.Second,
		CheckPoolTimeout:        3 * time.Minute,
		FlowControl:             false,

		Logger:      log,
		MetricsProv: &disabled.Provider{},
		Tracer:      trace.NewNoopTracerProvider().Tracer("hyperchain"),
		DelFlag:     make(chan bool),
	}
}

// newTestNode init the testNode instance
func (tf *testFramework[T, Constraint]) newTestNode(id uint64, p2pNodeID string, cc chan *channelMsg) *testNode[T, Constraint] {
	// Init logger
	poolLog := logrus.New()
	poolLog.WithField("mode ", fmt.Sprintf("txpool%d", id))
	// Simulation Function of External
	var ext ExternalStack[T, Constraint]
	testExt := &testExternal[T, Constraint]{
		tf:                     tf,
		testNode:               nil,
		clusterChan:            cc,
		configCheckpointRecord: make(map[uint64]*consensus.EpochChange),
	}
	ext = testExt

	// Memool Instance, Parameters in Config are Flexible
	txpoolConfig := txpool.Config{
		PoolSize:            100000,
		BatchSize:           500,
		BatchMemLimit:       false,
		BatchMaxMem:         999,
		ToleranceTime:       999 * time.Millisecond,
		ToleranceRemoveTime: 15 * time.Minute,
		Logger:              poolLog,
		GetAccountNonce: func(address string) uint64 {
			return 0
		},
	}
	pool, err := txpool.NewTxPool[T, Constraint](txpoolConfig)
	if err != nil {
		panic(err)
	}

	log := common.NewSimpleLogger()
	log.SetPrefix(fmt.Sprintf("[node%d] ", id))
	conf := tf.newNodeConfig(p2pNodeID, log, 1)
	n, err := newNode[T, Constraint](conf, ext, pool, true)
	if err != nil {
		panic(err)
	}
	// init new view.
	n.rbft.vcMgr.latestNewView = initialNewView
	tn := &testNode[T, Constraint]{
		Logger:     log,
		N:          n,
		n:          n,
		Router:     peerSet,
		Epoch:      0,
		ID:         id,
		P2PNodeID:  p2pNodeID,
		normal:     true,
		online:     true,
		close:      make(chan bool),
		recvChan:   make(chan *consensusMessageWrapper),
		stateStore: make(map[string][]byte),
		blocks:     make(map[uint64]string),
	}
	testExt.testNode = tn

	// Init State
	stateInit := &types.ServiceState{}
	stateInit.MetaState = &types.MetaState{
		Height: 0,
		Digest: "XXX GENESIS",
	}
	tn.N.ReportExecuted(stateInit)

	return tn
}

// =============================================================================
// general process method
// =============================================================================

func newTx() *consensus.FltTransaction {
	randomBytes := make([]byte, 20)

	_, err := rand.Read(randomBytes)
	if err != nil {
		panic(err)
	}
	from := hex.EncodeToString(randomBytes)
	return &consensus.FltTransaction{
		From:      []byte(from),
		Value:     []byte(string(rune(rand.Int()))),
		Nonce:     0,
		Timestamp: time.Now().UnixNano(),
	}
}

// =============================================================================
// External Interface Implement
// =============================================================================
// Storage
func (ext *testExternal[T, Constraint]) StoreState(key string, value []byte) error {
	ext.testNode.stateStore[key] = value
	return nil
}

func (ext *testExternal[T, Constraint]) DelState(key string) error {
	delete(ext.testNode.stateStore, key)
	if ext.testNode.stateStore == nil {
		ext.testNode.stateStore = make(map[string][]byte)
	}
	return nil
}

func (ext *testExternal[T, Constraint]) ReadState(key string) ([]byte, error) {
	value := ext.testNode.stateStore[key]

	if value != nil {
		return value, nil
	}

	return nil, errors.New("empty")
}

func (ext *testExternal[T, Constraint]) ReadStateSet(key string) (map[string][]byte, error) {
	value := ext.testNode.stateStore[key]

	if value != nil {
		ret := map[string][]byte{
			key: value,
		}
		return ret, nil
	}

	return nil, errors.New("empty")
}

func (ext *testExternal[T, Constraint]) Destroy(_ string) error {
	return nil
}

// Network
func (ext *testExternal[T, Constraint]) postMsg(msg *channelMsg) {
	ext.clusterChan <- msg
}

func (ext *testExternal[T, Constraint]) ListenMsg() <-chan *channelMsg {
	return ext.clusterChan
}

func (ext *testExternal[T, Constraint]) Broadcast(ctx context.Context, msg *consensus.ConsensusMessage) error {
	if !ext.testNode.online {
		return errors.New("node offline")
	}

	cm := &channelMsg{
		msg: msg,
		to:  "",
	}
	ext.tf.log.Infof("[node%d] Broadcast msg %v", ext.testNode.ID, msg.Type)
	ext.testNode.broadcastMessageCache = &consensusMessageWrapper{
		ctx:              ctx,
		ConsensusMessage: msg,
	}

	go ext.postMsg(cm)
	return nil
}

func (ext *testExternal[T, Constraint]) Unicast(ctx context.Context, msg *consensus.ConsensusMessage, to string) error {
	if !ext.testNode.online {
		return errors.New("node offline")
	}

	cm := &channelMsg{
		msg: msg,
	}

	for _, peer := range ext.testNode.Router {
		if peer.P2PNodeID == to {
			cm.to = peer.P2PNodeID
			break
		}
	}
	ext.testNode.unicastMessageCache = &consensusMessageWrapper{
		ctx:              ctx,
		ConsensusMessage: msg,
	}

	go ext.postMsg(cm)
	return nil
}

// Crypto
func (ext *testExternal[T, Constraint]) Sign(msg []byte) ([]byte, error) {
	return nil, nil
}

func (ext *testExternal[T, Constraint]) Verify(_ string, _ []byte, _ []byte) error {
	return nil
}

// ServiceOutbound
func (ext *testExternal[T, Constraint]) Execute(requests []*T, _ []bool, seqNo uint64, timestamp int64, _ string) {
	var txHashList []string
	for _, req := range requests {
		txHash := Constraint(req).RbftGetTxHash()
		txHashList = append(txHashList, txHash)
	}
	blockHash := calculateMD5Hash(txHashList, timestamp)

	state := &types.ServiceState{}
	state.MetaState = &types.MetaState{
		Height: seqNo,
		Digest: blockHash,
	}

	if state.MetaState.Height == ext.testNode.Applied+1 {
		ext.testNode.Applied = state.MetaState.Height
		ext.testNode.Digest = state.MetaState.Digest
		ext.testNode.blocks[state.MetaState.Height] = state.MetaState.Digest
		ext.testNode.n.logger.Debugf("Block Number %d", state.MetaState.Height)
		ext.testNode.n.logger.Debugf("Block Hash %s", state.MetaState.Digest)
		// report latest validator set

		go ext.testNode.N.ReportExecuted(state)
	}
}

func (ext *testExternal[T, Constraint]) StateUpdate(localLowWatermark, seqNo uint64, digest string, signedCheckpoints []*consensus.SignedCheckpoint, epochChanges ...*consensus.EpochChange) {
	localApplyId := ext.testNode.Applied
	updateCount := uint64(0)
	for key, val := range ext.tf.TestNode[0].blocks {
		if key <= seqNo && key > localApplyId {
			ext.testNode.blocks[key] = val
			ext.testNode.n.logger.Debugf("Block Number %d", key)
			ext.testNode.n.logger.Debugf("Block Hash %s", val)

			updateCount++
		}
	}
	if updateCount == seqNo-localApplyId {
		ext.testNode.Applied = seqNo
		ext.testNode.Digest = ext.testNode.blocks[seqNo]
	}
	ext.testNode.Epoch = ext.tf.TestNode[0].Epoch

	var checkpoint *consensus.Checkpoint
	signatures := make(map[uint64][]byte, len(signedCheckpoints))
	for _, signedCheckpoint := range signedCheckpoints {
		if checkpoint == nil {
			checkpoint = signedCheckpoint.Checkpoint
		} else {
			if !checkpoint.Equals(signedCheckpoint.Checkpoint) {
				ext.testNode.n.logger.Errorf("inconsistent checkpoint, one: %+v, another: %+v",
					checkpoint, signedCheckpoint.Checkpoint)
				return
			}
		}
		signatures[signedCheckpoint.Author] = signedCheckpoint.Signature
	}

	for _, ec := range epochChanges {
		ext.configCheckpointRecord[ec.Checkpoint.Epoch()] = ec
	}
	quorumCheckpoint := &consensus.QuorumCheckpoint{
		Checkpoint: checkpoint,
		Signatures: signatures,
	}
	ext.lastConfigCheckpoint = quorumCheckpoint

	state := &types.ServiceState{}
	state.Epoch = ext.testNode.n.config.GenesisEpochInfo.Epoch
	state.MetaState = &types.MetaState{
		Height: ext.testNode.Applied,
		Digest: ext.testNode.Digest,
	}
	ext.tf.log.Infof("[node%d] Report state updated state: %+v", ext.testNode.ID, state)
	ext.testNode.N.ReportStateUpdated(&types.ServiceSyncState{
		ServiceState: *state,
		EpochChanged: false,
	})
}

func (ext *testExternal[T, Constraint]) SendFilterEvent(informType types.InformType, message ...any) {
	switch informType {
	case types.InformTypeFilterStableCheckpoint:
		signedCheckpoints, ok := message[0].([]*consensus.SignedCheckpoint)
		if !ok {
			return
		}
		var checkpoint *consensus.Checkpoint
		signatures := make(map[uint64][]byte, len(signedCheckpoints))
		for _, signedCheckpoint := range signedCheckpoints {
			if checkpoint == nil {
				checkpoint = signedCheckpoint.Checkpoint
			} else {
				if !checkpoint.Equals(signedCheckpoint.Checkpoint) {
					ext.testNode.n.logger.Errorf("inconsistent checkpoint, one: %+v, another: %+v",
						checkpoint, signedCheckpoint.Checkpoint)
					return
				}
			}
			signatures[signedCheckpoint.Author] = signedCheckpoint.Signature
		}

		quorumCheckpoint := &consensus.QuorumCheckpoint{
			Checkpoint: checkpoint,
			Signatures: signatures,
		}

		validator := make([]string, len(peerSet))
		for i, p := range peerSet {
			validator[i] = p.P2PNodeID
		}

		epochChange := &consensus.EpochChange{
			Checkpoint: quorumCheckpoint,
			Validators: validator,
		}

		if quorumCheckpoint.NeedUpdateEpoch() {
			ext.lastConfigCheckpoint = quorumCheckpoint
			ext.configCheckpointRecord[quorumCheckpoint.Epoch()] = epochChange
			ext.testNode.n.logger.Noticef("update latest checkpoint to epoch: %d, height: %d",
				quorumCheckpoint.Epoch(), quorumCheckpoint.Height())
		}
	}
}

// TODO: supported epoch change
func (ext *testExternal[T, Constraint]) GetCurrentEpochInfo() (*EpochInfo, error) {
	return ext.testNode.n.config.GenesisEpochInfo, nil
}

func (ext *testExternal[T, Constraint]) GetEpochInfo(epoch uint64) (*EpochInfo, error) {
	return ext.testNode.n.config.GenesisEpochInfo, nil
}

func (ext *testExternal[T, Constraint]) StoreEpochState(key string, value []byte) error {
	return nil
}

func (ext *testExternal[T, Constraint]) ReadEpochState(key string) ([]byte, error) {
	return nil, nil
}
