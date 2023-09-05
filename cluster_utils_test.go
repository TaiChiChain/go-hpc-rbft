package rbft

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/axiomesh/axiom-bft/common/consensus"
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

// ******************************************************************************************************************
// assume that, it is in view 0, and the primary is node1, then we will process transactions:
// 1) "execute", we process transactions normally
// 2) "executeExceptPrimary", the backup replicas will process tx, but the primary only send a pre-prepare message
// 3) "executeExceptN", the node "N" will be abnormal and others will process transactions normally
// ******************************************************************************************************************
func execute[T any, Constraint consensus.TXConstraint[T]](t *testing.T, rbfts []*rbftImpl[T, Constraint], nodes []*testNode[T, Constraint], tx *T, checkpoint bool) map[consensus.Type][]*consensusMessageWrapper {
	return executeExceptN[T, Constraint](t, rbfts, nodes, tx, checkpoint, len(rbfts))
}

func executeExceptPrimary[T any, Constraint consensus.TXConstraint[T]](t *testing.T, rbfts []*rbftImpl[T, Constraint], nodes []*testNode[T, Constraint], tx *T, checkpoint bool) map[consensus.Type][]*consensusMessageWrapper {
	return executeExceptN[T, Constraint](t, rbfts, nodes, tx, checkpoint, 0)
}

func executeExceptN[T any, Constraint consensus.TXConstraint[T]](t *testing.T, rbfts []*rbftImpl[T, Constraint], nodes []*testNode[T, Constraint], tx *T, checkpoint bool, notExec int) map[consensus.Type][]*consensusMessageWrapper {
	var primaryIndex int
	for index := range rbfts {
		if index == notExec {
			continue
		}
		if rbfts[index].isPrimary(rbfts[index].peerMgr.selfID) {
			primaryIndex = index
			break
		}
	}
	retMessages := make(map[consensus.Type][]*consensusMessageWrapper)

	rbfts[0].batchMgr.requestPool.AddNewRequests([]*T{tx}, false, true, false)
	rbfts[1].batchMgr.requestPool.AddNewRequests([]*T{tx}, false, true, false)
	rbfts[2].batchMgr.requestPool.AddNewRequests([]*T{tx}, false, true, false)
	rbfts[3].batchMgr.requestPool.AddNewRequests([]*T{tx}, false, true, false)

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
		t.Logf("%d process pre-prepare", rbfts[index].peerMgr.selfID)
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

	return retMessages
}

// =============================================================================
// Tools for Cluster Check Stable State
// =============================================================================

// newRawLogger create log file for local cluster tests
func newRawLogger() *testLogger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02T15:04:05.000",
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			_, filename := filepath.Split(f.File)
			return "", fmt.Sprintf("%12s:%-4d", filename, f.Line)
		},
	})
	logger.SetReportCaller(false)
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.DebugLevel)
	return &testLogger{
		FieldLogger: logger,
	}
}

// set N/f of cluster
func (tf *testFramework[T, Constraint]) setN(num int) {
	tf.N = num
}

func newBasicClusterInstance[T any, Constraint consensus.TXConstraint[T]]() ([]*testNode[T, Constraint], []*rbftImpl[T, Constraint]) {
	tf := newTestFramework[T, Constraint](4)
	var rbfts []*rbftImpl[T, Constraint]
	var nodes []*testNode[T, Constraint]
	for _, tn := range tf.TestNode {
		rbfts = append(rbfts, tn.n.rbft)
		nodes = append(nodes, tn)
		err := tn.n.rbft.init()
		if err != nil {
			panic(err)
		}
		err = tn.n.rbft.batchMgr.requestPool.Start()
		if err != nil {
			panic(err)
		}
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
		assert.Equal(t, uint64(1), rbfts[index].chainConfig.View)
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
		From:    rbft.peerMgr.selfID,
		Epoch:   rbft.chainConfig.EpochInfo.Epoch,
		View:    rbft.chainConfig.View,
		Payload: payload,
	}

	return consensusMsg
}

func (tm *timerManager) getTimer(name string) bool {
	return tm.tTimers[name].count() != 0
}

// checkNilElems checks if provided struct has nil elements, returns error if provided
// param is not a struct pointer and returns all nil elements' name if has.
func checkNilElems(i any) (string, []string, error) {
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
