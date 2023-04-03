package rbft

import (
	"encoding/json"
	"errors"
	"math/rand"
	"strconv"
	"time"

	"github.com/hyperchain/go-hpc-common/fancylogger"
	"github.com/hyperchain/go-hpc-common/metrics/disabled"
	hpcCommonTypes "github.com/hyperchain/go-hpc-common/types"
	"github.com/hyperchain/go-hpc-common/types/protos"
	"github.com/hyperchain/go-hpc-rbft/v2/external"
	pb "github.com/hyperchain/go-hpc-rbft/v2/rbftpb"
	"github.com/hyperchain/go-hpc-rbft/v2/types"
	txpool "github.com/hyperchain/go-hpc-txpool"
)

var (
	defaultValidatorSet = []*protos.NodeInfo{
		{Hostname: "node1", PubKey: []byte("pub-1")},
		{Hostname: "node2", PubKey: []byte("pub-2")},
		{Hostname: "node3", PubKey: []byte("pub-3")},
		{Hostname: "node4", PubKey: []byte("pub-4")},
	}
)

var peerSet = []*types.Peer{
	{
		ID:       uint64(1),
		Hostname: "node1",
		Hash:     calHash("node1"),
	},
	{
		ID:       uint64(2),
		Hostname: "node2",
		Hash:     calHash("node2"),
	},
	{
		ID:       uint64(3),
		Hostname: "node3",
		Hash:     calHash("node3"),
	},
	{
		ID:       uint64(4),
		Hostname: "node4",
		Hash:     calHash("node4"),
	},
}

// testFramework contains the core structure of test framework instance.
type testFramework struct {
	N int

	// Instance of nodes.
	TestNode []*testNode

	// testFramework.Peers is the router map in cluster.
	// we could regard it as the routerInfo of epoch in trusted node
	Router types.Router

	// Channel to close this event process.
	close chan bool

	// Channel to receive messages sent from nodes in cluster.
	clusterChan chan *channelMsg

	// delFlag
	delFlag chan bool

	// Write logger to record some info.
	log Logger
}

// testNode contains the parameters of one node instance.
type testNode struct {
	// Node is provided for application to contact wih RBFT core.
	N Node

	// n is provided for testers to check RBFT core info.
	// Generally not used.
	n *node

	// testNode.Peers is the router map in one node.
	Router types.Router

	// epoch info in mem-chain
	Epoch uint64
	VSet  []*protos.NodeInfo

	// last config transaction in mem-chain
	Applied uint64
	Digest  string

	// block storage
	blocks map[uint64]string

	// ID is the num-identity of the local rbft.peerPool.localIDde.
	ID       uint64
	Hash     string
	Hostname string

	// Normal indicates if current node could deal with messages.
	normal bool

	// Online indicates if current node could receive or send messages.
	online bool

	// Channel to close this event process.
	close chan bool

	// Channel to receive messages in cluster.
	recvChan chan *pb.ConsensusMessage

	// Storage of consensus logs.
	stateStore map[string][]byte

	// consensus message cache for broadcast
	broadcastMessageCache *pb.ConsensusMessage

	// consensus message cache for unicast
	unicastMessageCache *pb.ConsensusMessage
}

// testExternal is the instance of External interface.
type testExternal struct {
	tf *testFramework

	// testNode indicates which node the Service belongs to.
	testNode *testNode

	// Channel to receive messages sent from nodes in cluster.
	clusterChan chan *channelMsg

	// mem chain for framework
	lastConfigCheckpoint *protos.QuorumCheckpoint

	// config checkpoint record
	configCheckpointRecord map[uint64]*protos.QuorumCheckpoint
}

// channelMsg is the form of data in cluster network.
type channelMsg struct {
	// target node of consensus message.
	// if it is "", the channelMsg is a broadcast message.
	// else it means the target node to unicast message.
	to string

	// consensus message
	msg *pb.ConsensusMessage
}

// defaultTxpoolSupportSmokeTest
type defaultTxpoolSupportSmokeTest struct{}

func (lookup *defaultTxpoolSupportSmokeTest) IsRequestsExist(txs []*protos.Transaction) []bool {
	results := make([]bool, len(txs))
	for i := range results {
		results[i] = false
	}
	return results
}

func (lookup *defaultTxpoolSupportSmokeTest) CheckSigns(txs []*protos.Transaction) {
}

//=============================================================================
// init process
//=============================================================================
// newTestFramework init the testFramework instance
func newTestFramework(account int, loggerFile bool) *testFramework {
	// Init PeerSet
	routers := types.Router{}
	for i := 0; i < account; i++ {
		id := uint64(i + 1)
		hostname := "node" + strconv.Itoa(i+1)
		peer := &types.Peer{
			ID:       id,
			Hash:     calHash(hostname),
			Hostname: hostname,
		}
		routers.Peers = append(routers.Peers, peer)
	}

	cc := make(chan *channelMsg, 1)
	// Init Framework
	delFlag := make(chan bool)
	tf := &testFramework{
		TestNode: nil,
		Router:   routers,

		close:       make(chan bool),
		clusterChan: cc,

		delFlag: delFlag,

		log: newRawLogger(),
	}

	tf.log.Debugf("routers:")
	for _, peer := range tf.Router.Peers {
		tf.log.Debugf("ID: %d, hostname: %s, hash: %s", peer.ID, peer.Hostname, peer.Hash)
	}

	// set node number
	tf.setN(len(tf.Router.Peers))

	// Init testNode in TestFramework
	for i := range tf.Router.Peers {
		tn := tf.newTestNode(tf.Router.Peers[i].ID, tf.Router.Peers[i].Hostname, cc, loggerFile)
		tf.TestNode = append(tf.TestNode, tn)
	}

	return tf
}

// newNodeConfig init the Config of Node.
func (tf *testFramework) newNodeConfig(
	id uint64,
	hostname string,
	log Logger,
	ext external.ExternalStack,
	pool txpool.TxPool,
	epoch uint64,
	vSet []*protos.NodeInfo,
	lastConfig *types.MetaState) Config {

	if len(vSet) < 4 {
		vSet = defaultValidatorSet
	}

	var peers []*types.Peer
	for index, info := range vSet {
		peer := &types.Peer{
			ID:       uint64(index + 1),
			Hostname: info.Hostname,
			Hash:     calHash(info.Hostname),
		}
		peers = append(peers, peer)
	}

	return Config{
		ID:       id,
		Hash:     calHash(hostname),
		Hostname: hostname,

		EpochInit:    epoch,
		Peers:        peers,
		LatestConfig: lastConfig,

		K:                       10,
		LogMultiplier:           4,
		SetSize:                 25,
		SetTimeout:              100 * time.Millisecond,
		BatchTimeout:            500 * time.Millisecond,
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
		External:    ext,
		RequestPool: pool,
		MetricsProv: &disabled.Provider{},
		DelFlag:     make(chan bool),
	}
}

// newTestNode init the testNode instance
func (tf *testFramework) newTestNode(id uint64, hostname string, cc chan *channelMsg, loggerFile bool) *testNode {
	// Init logger
	var log *fancylogger.Logger
	if loggerFile {
		log = newRawLoggerWithHost(hostname)
	} else {
		log = newRawLogger()
	}

	// Simulation Function of External
	var ext external.ExternalStack
	testExt := &testExternal{
		tf:                     tf,
		testNode:               nil,
		clusterChan:            cc,
		configCheckpointRecord: make(map[uint64]*protos.QuorumCheckpoint),
	}
	ext = testExt

	// TxPool Instance, Parameters in Config are Flexible
	confTxPool := txpool.Config{
		PoolSize:      100000,
		BatchSize:     4,
		BatchMemLimit: false,
		BatchMaxMem:   999,
		ToleranceTime: 999 * time.Millisecond,
		MetricsProv:   &disabled.Provider{},
		Logger:        log,
	}
	dtps := &defaultTxpoolSupportSmokeTest{}
	namespace := "global"
	pool := txpool.NewTxPool(namespace, dtps, confTxPool)
	conf := tf.newNodeConfig(id, hostname, log, ext, pool, 1, defaultValidatorSet, nil)
	n, _ := newNode(conf)
	// init new view.
	n.rbft.vcMgr.latestNewView = initialNewView
	routers := vSetToRouters(defaultValidatorSet)
	tn := &testNode{
		N:          n,
		n:          n,
		Router:     routers,
		Epoch:      0,
		ID:         id,
		Hash:       calHash(hostname),
		Hostname:   hostname,
		normal:     true,
		online:     true,
		close:      make(chan bool),
		recvChan:   make(chan *pb.ConsensusMessage),
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
// clusterListen listens and process the messages sent from nodes.
func (tf *testFramework) clusterListen() {
	for {
		select {
		case <-tf.close:
			return
		case obj := <-tf.clusterChan:
			for i := range tf.TestNode {
				if obj.to == "" || obj.to == tf.TestNode[i].Hash {
					if tf.TestNode[i].online && obj.msg.From != tf.TestNode[i].ID {
						tf.TestNode[i].recvChan <- obj.msg
					}
				}
			}
		}
	}
}

// nodeListen listens and process the messages received from cluster.
func (tn *testNode) nodeListen() {
	for {
		if tn.online == false {
			tn.N.Stop()
		}

		select {
		case <-tn.close:
			return
		case msg := <-tn.recvChan:
			if tn.normal {
				tn.N.Step(msg)
			}
		}
	}
}

// frameworkStart starts the cluster of RBFT test nodes.
func (tf *testFramework) frameworkStart() {
	tf.log.Notice("Test Framework starting...")

	go tf.clusterListen()

	for i := range tf.TestNode {
		_ = tf.TestNode[i].N.Start()
		go tf.TestNode[i].nodeListen()
	}
}

// startNode starts node according to id
func (tf *testFramework) startNode(id uint64) {
	for i := range tf.TestNode {
		if uint64(i) == id {
			tf.TestNode[i].online = true
			_ = tf.TestNode[i].N.Start()
			go tf.TestNode[i].nodeListen()
		}
	}
}

// stopNode stops node according to id
func (tf *testFramework) stopNode(id uint64) {
	for i := range tf.TestNode {
		if uint64(i) == id {
			if tf.TestNode[i].close != nil {
				close(tf.TestNode[i].close)
				tf.TestNode[i].close = nil
			}

			tf.TestNode[i].online = false
			tf.TestNode[i].N.Stop()
		}
	}
}

// frameworkStop stops the cluster of RBFT test nodes.
func (tf *testFramework) frameworkStop() {
	tf.log.Notice("Test Framework stopping...")

	for i := range tf.TestNode {
		if tf.TestNode[i].close != nil {
			close(tf.TestNode[i].close)
			tf.TestNode[i].close = nil
		}
	}

	if tf.close != nil {
		close(tf.close)
		tf.close = nil
	}

	for i := range tf.TestNode {
		tf.TestNode[i].online = false
		tf.TestNode[i].N.Stop()
	}
	tf.log.Noticef("======== Every Node in Cluster stopped!")
}

// frameworkDelNode adds one node to the cluster.
func (tf *testFramework) frameworkDelNode(hostname string) {
	hash := calHash(hostname)
	tf.log.Noticef("Test Framework delete Node, Hash: %s...", hostname)

	// stop node id
	for i, node := range tf.TestNode {
		if node.Hostname == hostname {
			tf.TestNode[i].online = false
			tf.TestNode[i].N.Stop()
		}
	}

	// send config change tx
	go func() {
		var (
			senderID     uint64
			senderHost   string
			senderIndex  int
			senderRouter types.Router
		)

		senderID = uint64(2)
		delRouter := types.Router{}
		for index, node := range tf.TestNode {
			if node.ID == senderID {
				senderHost = node.Hostname
				senderIndex = index
				senderRouter = getRouter(&node.Router)
				break
			}
		}

		delPeer := types.Peer{
			Hash:     hash,
			Hostname: hostname,
		}

		index := uint64(0)
		for _, node := range senderRouter.Peers {
			if node.Hostname != hostname {
				index++
				node.ID = index
				delRouter.Peers = append(delRouter.Peers, node)
			} else {
				delPeer.ID = node.ID
			}
		}

		tf.log.Infof("[Cluster_Service %s] Del Node Req: %+v", senderHost, delPeer)
		tf.log.Infof("[Cluster_Service %s] Target Validator Set: %+v", senderHost, delRouter)

		info, _ := json.Marshal(&delRouter)
		tx := &protos.Transaction{
			Timestamp: time.Now().UnixNano(),
			Id:        2,
			Nonce:     999,

			// use new router as value
			Value:  info,
			TxType: protos.Transaction_CTX,
		}
		txSet := &pb.RequestSet{
			Requests: []*protos.Transaction{tx},
			Local:    true,
		}
		_ = tf.TestNode[senderIndex].N.Propose(txSet)
	}()
}

// frameworkAddNode adds one node to the cluster.
func (tf *testFramework) frameworkAddNode(hostname string, loggerFile bool, vSet []*protos.NodeInfo) {
	tf.log.Noticef("Test Framework add Node, Host: %s...", hostname)

	maxID := uint64(0)
	for _, node := range tf.TestNode {
		if node.online {
			maxID++
		}
	}
	id := maxID + 1

	// Init logger
	var log *fancylogger.Logger
	if loggerFile {
		log = newRawLoggerWithHost(hostname)
	} else {
		log = newRawLogger()
	}

	// Simulation Function of External
	var ext external.ExternalStack
	testExt := &testExternal{
		tf:                     tf,
		testNode:               nil,
		clusterChan:            tf.clusterChan,
		configCheckpointRecord: make(map[uint64]*protos.QuorumCheckpoint),
	}
	ext = testExt

	// TxPool Instance, Parameters in Config are Flexible
	confTxPool := txpool.Config{
		PoolSize:      100000,
		BatchSize:     2,
		BatchMemLimit: false,
		BatchMaxMem:   999,
		ToleranceTime: 999 * time.Millisecond,
		MetricsProv:   &disabled.Provider{},
		Logger:        log,
	}
	dtps := &defaultTxpoolSupportSmokeTest{}
	namespace := "global"
	pool := txpool.NewTxPool(namespace, dtps, confTxPool)

	// new peer
	hash := calHash(hostname)
	newPeer := &types.Peer{
		ID:       id,
		Hash:     hash,
		Hostname: hostname,
	}

	// Add Peer Info to Framework
	var (
		senderHost  string
		senderIndex int
	)
	conf := tf.newNodeConfig(id, hostname, log, ext, pool, 1, vSet, nil)
	n, _ := newNode(conf)
	N := n

	routers := vSetToRouters(vSet)
	tn := &testNode{
		N:        N,
		n:        n,
		Router:   routers,
		Epoch:    n.rbft.epoch,
		ID:       id,
		Hash:     hash,
		Hostname: hostname,
		normal:   true,
		online:   true,
		close:    make(chan bool),
		recvChan: make(chan *pb.ConsensusMessage),
		blocks:   make(map[uint64]string),
	}
	tf.TestNode = append(tf.TestNode, tn)
	testExt.testNode = tn

	// Init State
	stateInit := &types.ServiceState{}
	stateInit.MetaState = &types.MetaState{
		Height: 0,
		Digest: "XXX GENESIS",
	}
	tn.N.ReportExecuted(stateInit)

	// send config change tx
	// node4 propose a ctx to add node
	go func() {
		tf.log.Infof("[Cluster_Service %d:%s] Add Node Req: %s", senderIndex, senderHost, newPeer.Hash)
		tf.log.Infof("[Cluster_Service %d:%s] Target Validator Set: %+v", senderIndex, senderHost, vSet)

		info, _ := json.Marshal(0)
		tx := &protos.Transaction{
			Timestamp: time.Now().UnixNano(),
			Id:        2,
			Nonce:     999,

			// use new router as value
			Value:  info,
			TxType: protos.Transaction_CTX,
		}
		txSet := &pb.RequestSet{
			Requests: []*protos.Transaction{tx},
			Local:    true,
		}
		_ = tf.TestNode[senderIndex].N.Propose(txSet)
	}()

	// Start the New Node listen, but don't start consensus
	go tn.nodeListen()
	go func() {
		_ = tn.N.Start()
	}()
}

func (tf *testFramework) sendTx(no int, sender uint64) {
	if sender == uint64(0) {
		sender = uint64(rand.Int()%tf.N + 1)
	}

	str3 := "tx" + string(rune(no))
	tx3 := &protos.Transaction{Value: []byte(str3)}
	txs3 := &pb.RequestSet{
		Requests: []*protos.Transaction{tx3},
		Local:    true,
	}
	_ = tf.TestNode[sender-1].N.Propose(txs3)
}

func newTx() *protos.Transaction {
	return &protos.Transaction{
		Value: []byte(string(rune(rand.Int()))),
		Nonce: int64(rand.Int()),
	}
}

func newCTX(vSet []*protos.NodeInfo) *protos.Transaction {
	info, _ := json.Marshal(vSet)
	return &protos.Transaction{
		Timestamp: time.Now().UnixNano(),
		Id:        2,
		Nonce:     999,
		// use new router as value
		Value:  info,
		TxType: protos.Transaction_CTX,
	}
}

func (tf *testFramework) sendInitCtx() {
	tf.log.Info("Send init ctx")
	senderID3 := uint64(rand.Int()%tf.N + 1)
	info, _ := json.Marshal(defaultValidatorSet)
	tx := &protos.Transaction{
		Timestamp: time.Now().UnixNano(),
		Id:        2,
		Nonce:     999,
		// use new router as value
		Value:  info,
		TxType: protos.Transaction_CTX,
	}
	txSet := &pb.RequestSet{
		Requests: []*protos.Transaction{tx},
		Local:    true,
	}
	_ = tf.TestNode[senderID3-1].N.Propose(txSet)
}

//=============================================================================
// External Interface Implement
//=============================================================================
// Storage
func (ext *testExternal) StoreState(key string, value []byte) error {
	ext.testNode.stateStore[key] = value
	return nil
}

func (ext *testExternal) DelState(key string) error {
	delete(ext.testNode.stateStore, key)
	if ext.testNode.stateStore == nil {
		ext.testNode.stateStore = make(map[string][]byte)
	}
	return nil
}

func (ext *testExternal) ReadState(key string) ([]byte, error) {
	value := ext.testNode.stateStore[key]

	if value != nil {
		return value, nil
	}

	return nil, errors.New("empty")
}

func (ext *testExternal) ReadStateSet(key string) (map[string][]byte, error) {
	value := ext.testNode.stateStore[key]

	if value != nil {
		ret := map[string][]byte{
			key: value,
		}
		return ret, nil
	}

	return nil, errors.New("empty")
}

func (ext *testExternal) Destroy(key string) error {
	return nil
}

// Network
func (ext *testExternal) postMsg(msg *channelMsg) {
	ext.clusterChan <- msg
}

func (ext *testExternal) Broadcast(msg *pb.ConsensusMessage) error {
	if !ext.testNode.online {
		return errors.New("node offline")
	}

	cm := &channelMsg{
		msg: msg,
		to:  "",
	}
	ext.tf.log.Debugf("%s broadcast msg %v", ext.testNode.Hostname, msg.Type)
	ext.testNode.broadcastMessageCache = msg

	go ext.postMsg(cm)
	return nil
}

func (ext *testExternal) Unicast(msg *pb.ConsensusMessage, to uint64) error {
	if !ext.testNode.online {
		return errors.New("node offline")
	}

	cm := &channelMsg{
		msg: msg,
	}

	for _, peer := range ext.testNode.Router.Peers {
		if peer.ID == to {
			cm.to = peer.Hash
			break
		}
	}
	ext.testNode.unicastMessageCache = msg

	go ext.postMsg(cm)
	return nil
}

func (ext *testExternal) UnicastByHostname(msg *pb.ConsensusMessage, to string) error {
	if !ext.testNode.online {
		return errors.New("node offline")
	}

	cm := &channelMsg{
		msg: msg,
		to:  to,
	}
	ext.testNode.unicastMessageCache = msg

	go ext.postMsg(cm)
	return nil
}

// Crypto
func (ext *testExternal) Sign(msg []byte) ([]byte, error) {
	return nil, nil
}

func (ext *testExternal) Verify(peerHash string, signature []byte, msg []byte) error {
	return nil
}

// ServiceOutbound
func (ext *testExternal) Execute(requests []*protos.Transaction, localList []bool, seqNo uint64, timestamp int64) {
	var txHashList []string
	for _, req := range requests {
		txHash := requestHash(req)
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
		//report latest validator set
		if len(requests) != 0 {
			if hpcCommonTypes.IsConfigTx(requests[0]) {
				var vSet []*protos.NodeInfo
				tmp := &vSet
				_ = json.Unmarshal(requests[0].Value, tmp)
				var peers []*types.Peer
				for index, nodeInfo := range vSet {
					peer := &types.Peer{
						ID:       uint64(index + 1),
						Hostname: nodeInfo.Hostname,
					}
					peers = append(peers, peer)
				}
				router := &types.Router{
					Peers: peers,
				}

				ext.testNode.Epoch = state.MetaState.Height
				ext.testNode.VSet = vSet

				ext.testNode.n.logger.Debugf("Validator Set %+v", router)
				ext.tf.log.Infof("router: %+v", router)
			}
		}
		go ext.testNode.N.ReportExecuted(state)

		if !hpcCommonTypes.IsConfigTx(requests[0]) && state.MetaState.Height%10 != 0 {
			success := make(chan bool)
			go func() {
				for {
					s := ext.testNode.n.getCurrentState()
					if s.MetaState.Height == state.MetaState.Height {
						success <- true
						break
					}
				}
			}()
			<-success
		}
	}
}

func (ext *testExternal) StateUpdate(seqNo uint64, digest string, signedCheckpoints []*pb.SignedCheckpoint) {
	for key, val := range ext.tf.TestNode[0].blocks {
		if key <= seqNo && key > ext.testNode.Applied {
			ext.testNode.blocks[key] = val
			ext.testNode.n.logger.Debugf("Block Number %d", key)
			ext.testNode.n.logger.Debugf("Block Hash %s", val)

			if key > ext.testNode.Applied {
				ext.testNode.Applied = key
				ext.testNode.Digest = val
			}
		}
	}
	ext.testNode.VSet = ext.tf.TestNode[0].VSet
	ext.testNode.Epoch = ext.tf.TestNode[0].Epoch

	var checkpoint *protos.Checkpoint
	signatures := make(map[string][]byte, len(signedCheckpoints))
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

	quorumCheckpoint := &protos.QuorumCheckpoint{
		Checkpoint: checkpoint,
		Signatures: signatures,
	}
	ext.lastConfigCheckpoint = quorumCheckpoint

	state := &types.ServiceState{}
	state.Epoch = ext.GetEpoch()
	state.MetaState = &types.MetaState{
		Height: ext.testNode.Applied,
		Digest: ext.testNode.Digest,
	}
	ext.tf.log.Infof("report state updated state: %+v", state)
	ext.testNode.N.ReportStateUpdated(state)
}

func (ext *testExternal) SendFilterEvent(informType types.InformType, message ...interface{}) {
	switch informType {
	case types.InformTypeFilterStableCheckpoint:
		signedCheckpoints, ok := message[0].([]*pb.SignedCheckpoint)
		if !ok {
			return
		}
		var checkpoint *protos.Checkpoint
		signatures := make(map[string][]byte, len(signedCheckpoints))
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

		quorumCheckpoint := &protos.QuorumCheckpoint{
			Checkpoint: checkpoint,
			Signatures: signatures,
		}
		if quorumCheckpoint.Reconfiguration() {
			ext.lastConfigCheckpoint = quorumCheckpoint
			ext.configCheckpointRecord[quorumCheckpoint.Epoch()] = quorumCheckpoint
			ext.testNode.n.logger.Noticef("update latest checkpoint to epoch: %d, height: %d",
				quorumCheckpoint.Epoch(), quorumCheckpoint.Height())
		}
		height := signedCheckpoints[0].Checkpoint.Height()
		if ext.testNode.n.rbft.atomicIn(InConfChange) {
			go ext.testNode.N.ReportStableCheckpointFinished(height)
		}
	}
}

func (ext *testExternal) Reconfiguration() uint64 {
	var newPeers []*types.Peer
	for index, peer := range ext.testNode.VSet {
		newPeer := &types.Peer{
			ID:       uint64(index + 1),
			Hostname: peer.Hostname,
			Hash:     calHash(peer.Hostname),
		}
		newPeers = append(newPeers, newPeer)
	}
	newRouter := &types.Router{Peers: newPeers}
	ext.testNode.Router = getRouter(newRouter)
	ext.tf.log.Noticef("[Cluster_Service %s update] router: %+v", ext.testNode.Hostname, newRouter)
	cc := &types.ConfState{QuorumRouter: newRouter}
	ext.testNode.N.ApplyConfChange(cc)
	return ext.GetEpoch()
}

func (ext *testExternal) GetNodeInfos() []*protos.NodeInfo {
	return ext.testNode.VSet
}

func (ext *testExternal) GetAlgorithmVersion() string {
	return "RBFT"
}

func (ext *testExternal) GetEpoch() uint64 {
	return ext.GetLastCheckpoint().NextEpoch()
}

func (ext *testExternal) IsConfigBlock(height uint64) bool {
	return false
}

// GetLastCheckpoint return the last QuorumCheckpoint in ledger
func (ext *testExternal) GetLastCheckpoint() *protos.QuorumCheckpoint {
	if ext.lastConfigCheckpoint != nil {
		return ext.lastConfigCheckpoint
	}
	return &protos.QuorumCheckpoint{
		Checkpoint: &protos.Checkpoint{
			Epoch:          ext.testNode.Epoch,
			ConsensusState: &protos.Checkpoint_ConsensusState{},
			ExecuteState: &protos.Checkpoint_ExecuteState{
				Height: 0,
				Digest: "XXX GENESIS",
			},
			NextEpochState: &protos.Checkpoint_NextEpochState{},
		},
		Signatures: nil,
	}
}

// GetCheckpointOfEpoch gets checkpoint of given epoch.
func (ext *testExternal) GetCheckpointOfEpoch(epoch uint64) (*protos.QuorumCheckpoint, error) {
	return ext.configCheckpointRecord[epoch], nil
}

// VerifyEpochChangeProof verifies the proof is correctly chained with known validator verifier.
func (ext *testExternal) VerifyEpochChangeProof(proof *protos.EpochChangeProof, validators protos.Validators) error {
	return nil
}
