package rbft

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/hyperchain/go-hpc-common/fancylogger"
	hpcCommonTypes "github.com/hyperchain/go-hpc-common/types"
	"github.com/hyperchain/go-hpc-common/types/protos"
	pb "github.com/hyperchain/go-hpc-rbft/v2/rbftpb"
	"github.com/hyperchain/go-hpc-rbft/v2/types"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

// ******************************************************************************************************************
// some tools for unit tests
// ******************************************************************************************************************
func unlockCluster(rbfts []*rbftImpl) {
	for index := range rbfts {
		rbfts[index].atomicOff(Pending)
		rbfts[index].setNormal()
	}
}

func setClusterViewExcept(rbfts []*rbftImpl, nodes []*testNode, view uint64, noSet int) {
	for index := range rbfts {
		if index == noSet {
			continue
		}
		rbfts[index].setView(view)
	}
}

func setClusterExec(rbfts []*rbftImpl, nodes []*testNode, seq uint64) {
	setClusterExecExcept(rbfts, nodes, seq, len(rbfts))
}

func setClusterExecExcept(rbfts []*rbftImpl, nodes []*testNode, seq uint64, noExec int) {
	for index := range rbfts {
		if index == noExec {
			continue
		}
		rbfts[index].exec.setLastExec(seq)
		rbfts[index].batchMgr.setSeqNo(seq)
		pos := seq / 10 * 10
		rbfts[index].moveWatermarks(pos, false)
		nodes[index].Applied = seq
	}
}

//******************************************************************************************************************
// assume that, it is in view 0, and the primary is node1, then we will process transactions:
// 1) "execute", we process transactions normally
// 2) "executeExceptPrimary", the backup replicas will process tx, but the primary only send a pre-prepare message
// 3) "executeExceptN", the node "N" will be abnormal and others will process transactions normally
//******************************************************************************************************************

func execute(t *testing.T, rbfts []*rbftImpl, nodes []*testNode, tx *protos.Transaction, checkpoint bool) map[pb.Type][]*consensusMessageWrapper {
	return executeExceptN(t, rbfts, nodes, tx, checkpoint, len(rbfts))
}

func executeExceptPrimary(t *testing.T, rbfts []*rbftImpl, nodes []*testNode, tx *protos.Transaction, checkpoint bool) map[pb.Type][]*consensusMessageWrapper {
	return executeExceptN(t, rbfts, nodes, tx, checkpoint, 0)
}

func executeExceptN(t *testing.T, rbfts []*rbftImpl, nodes []*testNode, tx *protos.Transaction, checkpoint bool, notExec int) map[pb.Type][]*consensusMessageWrapper {

	var primaryIndex int
	for index := range rbfts {
		if index == notExec {
			continue
		}
		if rbfts[index].isPrimary(rbfts[index].peerPool.ID) {
			primaryIndex = index
			break
		}
	}
	retMessages := make(map[pb.Type][]*consensusMessageWrapper)

	rbfts[0].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)
	rbfts[1].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)
	rbfts[2].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)
	rbfts[3].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)

	batchTimerEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}
	// primary start generate batch
	rbfts[primaryIndex].processEvent(batchTimerEvent)

	preprepMsg := nodes[primaryIndex].broadcastMessageCache
	assert.Equal(t, pb.Type_PRE_PREPARE, preprepMsg.Type)
	retMessages[pb.Type_PRE_PREPARE] = []*consensusMessageWrapper{preprepMsg}

	prepMsg := make([]*consensusMessageWrapper, len(rbfts))
	commitMsg := make([]*consensusMessageWrapper, len(rbfts))
	checkpointMsg := make([]*consensusMessageWrapper, len(rbfts))

	for index := range rbfts {
		if index == primaryIndex || index == notExec {
			continue
		}
		t.Logf("%s process pre-prepare", rbfts[index].peerPool.hostname)
		rbfts[index].processEvent(preprepMsg)
		prepMsg[index] = nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_PREPARE, prepMsg[index].Type)
	}
	// for pre-prepare message
	// a primary won't process a pre-prepare, so that the prepMsg[primaryIndex] will always be empty
	assert.Nil(t, prepMsg[primaryIndex])
	retMessages[pb.Type_PREPARE] = prepMsg

	for index := range rbfts {
		if index == notExec {
			continue
		}
		for j := range prepMsg {
			if j == primaryIndex || j == notExec {
				continue
			}
			rbfts[index].processEvent(prepMsg[j])
		}
		commitMsg[index] = nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_COMMIT, commitMsg[index].Type)
	}
	retMessages[pb.Type_COMMIT] = commitMsg

	for index := range rbfts {
		if index == notExec {
			continue
		}
		for j := range commitMsg {
			if j == notExec {
				continue
			}
			rbfts[index].processEvent(commitMsg[j])
		}
		if checkpoint {
			checkpointMsg[index] = nodes[index].broadcastMessageCache
			assert.Equal(t, pb.Type_SIGNED_CHECKPOINT, checkpointMsg[index].Type)
		}
	}
	retMessages[pb.Type_SIGNED_CHECKPOINT] = checkpointMsg

	if checkpoint {
		if hpcCommonTypes.IsConfigTx(tx) {
			vcMsg := make([]*consensusMessageWrapper, len(rbfts))
			epochChanged := false
			// process checkpoint, if epoch changed, trigger and process view change.
			for index := range rbfts {
				oldEpoch := rbfts[index].epoch
				if index == notExec {
					continue
				}
				for j := range checkpointMsg {
					if j == notExec {
						continue
					}
					rbfts[index].processEvent(checkpointMsg[j])
				}
				newEpoch := rbfts[index].epoch
				if oldEpoch != newEpoch {
					epochChanged = true
					t.Logf("%s epoch changed from %d to %d, should trigger vc",
						rbfts[index].peerPool.hostname, oldEpoch, newEpoch)
					vcMsg[index] = nodes[index].broadcastMessageCache
				}
			}
			retMessages[pb.Type_VIEW_CHANGE] = vcMsg

			// trigger view change after epoch change.
			if epochChanged {
				for index := range rbfts {
					if index == notExec {
						continue
					}
					for j := range vcMsg {
						if j == notExec {
							continue
						}
						rbfts[index].processEvent(vcMsg[j])
					}
				}
				// view change quorum event
				// new primary should be replica 2(view = 1)
				primaryIndex = 1
				quorumVC := &LocalEvent{Service: ViewChangeService, EventType: ViewChangeQuorumEvent}
				for index := range rbfts {
					if index == notExec {
						continue
					}
					finished := rbfts[index].processEvent(quorumVC)
					if index == primaryIndex {
						assert.Equal(t, ViewChangeDoneEvent, finished.(*LocalEvent).EventType)
					}
				}
				// new view message
				newView := nodes[1].broadcastMessageCache
				assert.Equal(t, pb.Type_NEW_VIEW, newView.Type)
				for index := range rbfts {
					if index == notExec || index == primaryIndex {
						continue
					}

					finished := rbfts[index].processEvent(newView)
					assert.Equal(t, ViewChangeDoneEvent, finished.(*LocalEvent).EventType)
				}

				// recovery done event
				done := &LocalEvent{Service: ViewChangeService, EventType: ViewChangeDoneEvent}
				for index := range rbfts {
					if index == notExec {
						continue
					}

					rbfts[index].processEvent(done)
				}
			}

		} else {
			for index := range rbfts {
				if index == notExec {
					continue
				}
				for j := range checkpointMsg {
					if j == notExec {
						continue
					}
					rbfts[index].processEvent(checkpointMsg[j])
				}
			}
		}
	}

	return retMessages
}

//=============================================================================
// Tools for Cluster Check Stable State
//=============================================================================

// newRawLoggerWithHost create log file for local cluster tests
func newRawLoggerWithHost(hostname string) *fancylogger.Logger {
	rawLogger := fancylogger.NewLogger("test", fancylogger.DEBUG)

	consoleFormatter := &fancylogger.StringFormatter{
		EnableColors:    true,
		TimestampFormat: "2006-01-02T15:04:05.000",
		IsTerminal:      true,
	}

	//test with logger files
	_ = os.Mkdir("testLogger", os.ModePerm)
	fileName := "testLogger/" + hostname + ".log"
	f, _ := os.Create(fileName)
	consoleBackend := fancylogger.NewIOBackend(consoleFormatter, f)

	rawLogger.SetBackends(consoleBackend)
	rawLogger.SetEnableCaller(true)

	return rawLogger
}

// newRawLogger create log file for local cluster tests
func newRawLogger() *fancylogger.Logger {
	rawLogger := fancylogger.NewLogger("test", fancylogger.DEBUG)

	consoleFormatter := &fancylogger.StringFormatter{
		EnableColors:    true,
		TimestampFormat: "2006-01-02T15:04:05.000",
		IsTerminal:      true,
	}

	consoleBackend := fancylogger.NewIOBackend(consoleFormatter, os.Stdout)

	rawLogger.SetBackends(consoleBackend)
	rawLogger.SetEnableCaller(true)

	return rawLogger
}

func vSetToRouters(vSet []*protos.NodeInfo) types.Router {
	var routers types.Router
	for index, info := range vSet {
		peer := &types.Peer{
			ID:       uint64(index + 1),
			Hash:     calHash(info.Hostname),
			Hostname: info.Hostname,
		}
		routers.Peers = append(routers.Peers, peer)
	}
	return routers
}

func getRouter(router *types.Router) types.Router {
	var r types.Router
	for _, p := range router.Peers {
		peer := types.Peer{
			ID:       p.ID,
			Hash:     p.Hash,
			Hostname: p.Hostname,
		}
		r.Peers = append(r.Peers, &peer)
	}
	return r
}

// set N/f of cluster
func (tf *testFramework) setN(num int) {
	tf.N = num
}

func calHash(hostname string) string {
	return hostname
}

func newBasicClusterInstance() ([]*testNode, []*rbftImpl) {
	tf := newTestFramework(4, false)
	var rbfts []*rbftImpl
	var nodes []*testNode
	for _, tn := range tf.TestNode {
		rbfts = append(rbfts, tn.n.rbft)
		nodes = append(nodes, tn)
		_ = tn.n.rbft.batchMgr.requestPool.Start()
	}

	return nodes, rbfts
}

// clusterInitRecovery mocks cluster init recovery and change view to 1.
// notExec indicates which node is offline and not execute recovery, use -1 to indicate no such nodes.
// NOTE!!! assume replica 2 is online(the primary in view 1).
func clusterInitRecovery(t *testing.T, allNodes []*testNode, allRbfts []*rbftImpl, notExec int) {
	nodes := make([]*testNode, 0)
	rbfts := make([]*rbftImpl, 0)
	for idx := range allNodes {
		if idx == notExec {
			continue
		}
		nodes = append(nodes, allNodes[idx])
		rbfts = append(rbfts, allRbfts[idx])
	}

	init := &LocalEvent{
		Service:   RecoveryService,
		EventType: RecoveryInitEvent,
		Event:     uint64(0),
	}

	// init recovery directly and broadcast recovery view change in view=1
	recoveryVCs := make([]*consensusMessageWrapper, 0, len(nodes))
	for idx := range rbfts {
		rbfts[idx].processEvent(init)
		recoveryVC := nodes[idx].broadcastMessageCache
		assert.Equal(t, pb.Type_VIEW_CHANGE, recoveryVC.Type)
		recoveryVCs = append(recoveryVCs, recoveryVC)
	}

	// process all recovery view changes and enter view change quorum status
	for idx := range rbfts {
		var quorum consensusEvent
		for i := range recoveryVCs {
			if i == idx {
				continue
			}
			quorum = rbfts[idx].processEvent(recoveryVCs[i])
		}
		assert.Equal(t, ViewChangeQuorumEvent, quorum.(*LocalEvent).EventType)
	}

	// process view change quorum event
	// NOTE!!! assume replica 2 is online.
	quorumVC := &LocalEvent{Service: ViewChangeService, EventType: ViewChangeQuorumEvent}
	for idx := range rbfts {
		finished := rbfts[idx].processEvent(quorumVC)
		// primary finish view change and broadcast new view.
		if idx == 1 {
			assert.Equal(t, ViewChangeDoneEvent, finished.(*LocalEvent).EventType)
		}
	}

	// new view message
	newView := nodes[1].broadcastMessageCache
	assert.Equal(t, pb.Type_NEW_VIEW, newView.Type)

	for index := range rbfts {
		if index == 1 {
			continue
		}

		// replicas process new view and finish view change.
		finished := rbfts[index].processEvent(newView)
		assert.Equal(t, ViewChangeDoneEvent, finished.(*LocalEvent).EventType)
	}

	// recovery done event
	done := &LocalEvent{Service: ViewChangeService, EventType: ViewChangeDoneEvent}
	for index := range rbfts {
		rbfts[index].processEvent(done)
	}

	// check status
	for index := range rbfts {
		assert.Equal(t, uint64(1), rbfts[index].view)
		assert.True(t, rbfts[index].isNormal())
	}
}

func (rbft *rbftImpl) consensusMessagePacker(e consensusEvent) *pb.ConsensusMessage {
	var (
		eventType pb.Type
		payload   []byte
		err       error
	)

	switch et := e.(type) {
	case *pb.NullRequest:
		eventType = pb.Type_NULL_REQUEST
		payload, err = proto.Marshal(et)
	case *pb.PrePrepare:
		eventType = pb.Type_PRE_PREPARE
		payload, err = proto.Marshal(et)
	case *pb.Prepare:
		eventType = pb.Type_PREPARE
		payload, err = proto.Marshal(et)
	case *pb.Commit:
		eventType = pb.Type_COMMIT
		payload, err = proto.Marshal(et)
	case *pb.RequestSet:
		eventType = pb.Type_REQUEST_SET
		payload, err = proto.Marshal(et)
	case *pb.SignedCheckpoint:
		eventType = pb.Type_SIGNED_CHECKPOINT
		payload, err = proto.Marshal(et)
	case *pb.FetchCheckpoint:
		eventType = pb.Type_FETCH_CHECKPOINT
		payload, err = proto.Marshal(et)
	case *pb.ViewChange:
		eventType = pb.Type_VIEW_CHANGE
		payload, err = proto.Marshal(et)
	case *pb.QuorumViewChange:
		eventType = pb.Type_QUORUM_VIEW_CHANGE
		payload, err = proto.Marshal(et)
	case *pb.NewView:
		eventType = pb.Type_NEW_VIEW
		payload, err = proto.Marshal(et)
	case *pb.FetchView:
		eventType = pb.Type_FETCH_VIEW
		payload, err = proto.Marshal(et)
	case *pb.RecoveryResponse:
		eventType = pb.Type_RECOVERY_RESPONSE
		payload, err = proto.Marshal(et)
	case *pb.FetchBatchRequest:
		eventType = pb.Type_FETCH_BATCH_REQUEST
		payload, err = proto.Marshal(et)
	case *pb.FetchBatchResponse:
		eventType = pb.Type_FETCH_BATCH_RESPONSE
		payload, err = proto.Marshal(et)
	case *pb.FetchPQCRequest:
		eventType = pb.Type_FETCH_PQC_REQUEST
		payload, err = proto.Marshal(et)
	case *pb.FetchPQCResponse:
		eventType = pb.Type_FETCH_PQC_RESPONSE
		payload, err = proto.Marshal(et)
	case *pb.FetchMissingRequest:
		eventType = pb.Type_FETCH_MISSING_REQUEST
		payload, err = proto.Marshal(et)
	case *pb.FetchMissingResponse:
		eventType = pb.Type_FETCH_MISSING_RESPONSE
		payload, err = proto.Marshal(et)
	case *pb.SyncState:
		eventType = pb.Type_SYNC_STATE
		payload, err = proto.Marshal(et)
	case *pb.SyncStateResponse:
		eventType = pb.Type_SYNC_STATE_RESPONSE
		payload, err = proto.Marshal(et)
	case *pb.EpochChangeRequest:
		eventType = pb.Type_EPOCH_CHANGE_REQUEST
		payload, err = proto.Marshal(et)
	case *protos.EpochChangeProof:
		eventType = pb.Type_EPOCH_CHANGE_PROOF
		payload, err = proto.Marshal(et)
	default:
		rbft.logger.Errorf("ConsensusMessage Unknown Type: %+v", e)
		return nil
	}

	if err != nil {
		rbft.logger.Errorf("ConsensusMessage Marshal Error: %s", err)
		return nil
	}

	consensusMsg := &pb.ConsensusMessage{
		Type:    eventType,
		From:    rbft.peerPool.ID,
		Author:  rbft.peerPool.hostname,
		Epoch:   rbft.epoch,
		View:    rbft.view,
		Payload: payload,
	}

	return consensusMsg
}

func (tm *timerManager) getTimer(name string) bool {
	return tm.tTimers[name].count() != 0
}

// checkNilElems checks if provided struct has nil elements, returns error if provided
// param is not a struct pointer and returns all nil elements' name if has.
func checkNilElems(i interface{}) (string, []string, error) {
	typ := reflect.TypeOf(i)
	value := reflect.Indirect(reflect.ValueOf(i))

	if typ.Kind() != reflect.Ptr {
		return "", nil, errors.New("got a non-ptr to check if has nil elements")
	}
	typ = typ.Elem()
	if typ.Kind() != reflect.Struct {
		return "", nil, errors.New("got a non-struct to check if has nil elements")
	}

	structName := typ.Name()
	nilElems := make([]string, 0)
	hasNil := false

	for i := 0; i < typ.NumField(); i++ {
		kind := typ.Field(i).Type.Kind()
		if kind == reflect.Chan || kind == reflect.Map {
			elemName := typ.Field(i).Name
			if value.FieldByName(elemName).IsNil() {
				nilElems = append(nilElems, elemName)
				hasNil = true
			}
		}
	}
	if hasNil {
		return structName, nilElems, nil
	}
	return structName, nil, nil
}
