package rbft

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/hyperchain/go-hpc-rbft/common/consensus"
	"github.com/hyperchain/go-hpc-rbft/common/fancylogger"
	"github.com/hyperchain/go-hpc-rbft/types"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

// ******************************************************************************************************************
// some tools for unit tests
// ******************************************************************************************************************
func unlockCluster[T any, Constraint consensus.TXConstraint[T]](rbfts []*rbftImpl[T, Constraint]) {
	for index := range rbfts {
		rbfts[index].atomicOff(Pending)
		rbfts[index].setNormal()
	}
}

func setClusterViewExcept[T any, Constraint consensus.TXConstraint[T]](rbfts []*rbftImpl[T, Constraint], nodes []*testNode[T, Constraint], view uint64, noSet int) {
	for index := range rbfts {
		if index == noSet {
			continue
		}
		rbfts[index].setView(view)
	}
}

func setClusterExec[T any, Constraint consensus.TXConstraint[T]](rbfts []*rbftImpl[T, Constraint], nodes []*testNode[T, Constraint], seq uint64) {
	setClusterExecExcept[T, Constraint](rbfts, nodes, seq, len(rbfts))
}

func setClusterExecExcept[T any, Constraint consensus.TXConstraint[T]](rbfts []*rbftImpl[T, Constraint], nodes []*testNode[T, Constraint], seq uint64, noExec int) {
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

func execute[T any, Constraint consensus.TXConstraint[T]](t *testing.T, rbfts []*rbftImpl[T, Constraint], nodes []*testNode[T, Constraint], tx *consensus.Transaction, checkpoint bool) map[consensus.Type][]*consensusMessageWrapper {
	return executeExceptN[T, Constraint](t, rbfts, nodes, tx, checkpoint, len(rbfts))
}

func executeExceptPrimary[T any, Constraint consensus.TXConstraint[T]](t *testing.T, rbfts []*rbftImpl[T, Constraint], nodes []*testNode[T, Constraint], tx *consensus.Transaction, checkpoint bool) map[consensus.Type][]*consensusMessageWrapper {
	return executeExceptN[T, Constraint](t, rbfts, nodes, tx, checkpoint, 0)
}

func executeExceptN[T any, Constraint consensus.TXConstraint[T]](t *testing.T, rbfts []*rbftImpl[T, Constraint], nodes []*testNode[T, Constraint], tx *consensus.Transaction, checkpoint bool, notExec int) map[consensus.Type][]*consensusMessageWrapper {

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
	retMessages := make(map[consensus.Type][]*consensusMessageWrapper)
	txBytes, err := tx.Marshal()
	assert.Nil(t, err)
	rbfts[0].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, false, true)
	rbfts[1].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, false, true)
	rbfts[2].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, false, true)
	rbfts[3].batchMgr.requestPool.AddNewRequests([][]byte{txBytes}, false, true)

	batchTimerEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}
	// primary start generate batch
	rbfts[primaryIndex].processEvent(batchTimerEvent)

	preprepMsg := nodes[primaryIndex].broadcastMessageCache
	assert.Equal(t, consensus.Type_PRE_PREPARE, preprepMsg.Type)
	retMessages[consensus.Type_PRE_PREPARE] = []*consensusMessageWrapper{preprepMsg}

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
		assert.Equal(t, consensus.Type_PREPARE, prepMsg[index].Type)
	}
	// for pre-prepare message
	// a primary won't process a pre-prepare, so that the prepMsg[primaryIndex] will always be empty
	assert.Nil(t, prepMsg[primaryIndex])
	retMessages[consensus.Type_PREPARE] = prepMsg

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
		assert.Equal(t, consensus.Type_COMMIT, commitMsg[index].Type)
	}
	retMessages[consensus.Type_COMMIT] = commitMsg

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
			assert.Equal(t, consensus.Type_SIGNED_CHECKPOINT, checkpointMsg[index].Type)
		}
	}
	retMessages[consensus.Type_SIGNED_CHECKPOINT] = checkpointMsg

	if checkpoint {
		txBytes, err := tx.Marshal()
		assert.Nil(t, err)
		if consensus.IsConfigTx[T, Constraint](txBytes) {
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
			retMessages[consensus.Type_VIEW_CHANGE] = vcMsg

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
				assert.Equal(t, consensus.Type_NEW_VIEW, newView.Type)
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

func vSetToRouters(vSet []*consensus.NodeInfo) types.Router {
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
func (tf *testFramework[T, Constraint]) setN(num int) {
	tf.N = num
}

func calHash(hostname string) string {
	return hostname
}

func newBasicClusterInstance[T any, Constraint consensus.TXConstraint[T]]() ([]*testNode[T, Constraint], []*rbftImpl[T, Constraint]) {
	tf := newTestFramework[T, Constraint](4, false)
	var rbfts []*rbftImpl[T, Constraint]
	var nodes []*testNode[T, Constraint]
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
func clusterInitRecovery[T any, Constraint consensus.TXConstraint[T]](t *testing.T, allNodes []*testNode[T, Constraint], allRbfts []*rbftImpl[T, Constraint], notExec int) {
	nodes := make([]*testNode[T, Constraint], 0)
	rbfts := make([]*rbftImpl[T, Constraint], 0)
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
		assert.Equal(t, consensus.Type_VIEW_CHANGE, recoveryVC.Type)
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
	assert.Equal(t, consensus.Type_NEW_VIEW, newView.Type)

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

func (rbft *rbftImpl[T, Constraint]) consensusMessagePacker(e consensusEvent) *consensus.ConsensusMessage {
	var (
		eventType consensus.Type
		payload   []byte
		err       error
	)

	switch et := e.(type) {
	case *consensus.NullRequest:
		eventType = consensus.Type_NULL_REQUEST
		payload, err = proto.Marshal(et)
	case *consensus.PrePrepare:
		eventType = consensus.Type_PRE_PREPARE
		payload, err = proto.Marshal(et)
	case *consensus.Prepare:
		eventType = consensus.Type_PREPARE
		payload, err = proto.Marshal(et)
	case *consensus.Commit:
		eventType = consensus.Type_COMMIT
		payload, err = proto.Marshal(et)
	case *consensus.RequestSet:
		eventType = consensus.Type_REQUEST_SET
		payload, err = proto.Marshal(et)
	case *consensus.SignedCheckpoint:
		eventType = consensus.Type_SIGNED_CHECKPOINT
		payload, err = proto.Marshal(et)
	case *consensus.FetchCheckpoint:
		eventType = consensus.Type_FETCH_CHECKPOINT
		payload, err = proto.Marshal(et)
	case *consensus.ViewChange:
		eventType = consensus.Type_VIEW_CHANGE
		payload, err = proto.Marshal(et)
	case *consensus.QuorumViewChange:
		eventType = consensus.Type_QUORUM_VIEW_CHANGE
		payload, err = proto.Marshal(et)
	case *consensus.NewView:
		eventType = consensus.Type_NEW_VIEW
		payload, err = proto.Marshal(et)
	case *consensus.FetchView:
		eventType = consensus.Type_FETCH_VIEW
		payload, err = proto.Marshal(et)
	case *consensus.RecoveryResponse:
		eventType = consensus.Type_RECOVERY_RESPONSE
		payload, err = proto.Marshal(et)
	case *consensus.FetchBatchRequest:
		eventType = consensus.Type_FETCH_BATCH_REQUEST
		payload, err = proto.Marshal(et)
	case *consensus.FetchBatchResponse:
		eventType = consensus.Type_FETCH_BATCH_RESPONSE
		payload, err = proto.Marshal(et)
	case *consensus.FetchPQCRequest:
		eventType = consensus.Type_FETCH_PQC_REQUEST
		payload, err = proto.Marshal(et)
	case *consensus.FetchPQCResponse:
		eventType = consensus.Type_FETCH_PQC_RESPONSE
		payload, err = proto.Marshal(et)
	case *consensus.FetchMissingRequest:
		eventType = consensus.Type_FETCH_MISSING_REQUEST
		payload, err = proto.Marshal(et)
	case *consensus.FetchMissingResponse:
		eventType = consensus.Type_FETCH_MISSING_RESPONSE
		payload, err = proto.Marshal(et)
	case *consensus.SyncState:
		eventType = consensus.Type_SYNC_STATE
		payload, err = proto.Marshal(et)
	case *consensus.SyncStateResponse:
		eventType = consensus.Type_SYNC_STATE_RESPONSE
		payload, err = proto.Marshal(et)
	case *consensus.EpochChangeRequest:
		eventType = consensus.Type_EPOCH_CHANGE_REQUEST
		payload, err = proto.Marshal(et)
	case *consensus.EpochChangeProof:
		eventType = consensus.Type_EPOCH_CHANGE_PROOF
		payload, err = proto.Marshal(et)
	default:
		rbft.logger.Errorf("ConsensusMessage Unknown Type: %+v", e)
		return nil
	}

	if err != nil {
		rbft.logger.Errorf("ConsensusMessage Marshal Error: %s", err)
		return nil
	}

	consensusMsg := &consensus.ConsensusMessage{
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
