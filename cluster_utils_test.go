package rbft

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/hyperchain/go-hpc-common/fancylogger"
	hpcCommonTypes "github.com/hyperchain/go-hpc-common/types"
	"github.com/hyperchain/go-hpc-common/types/protos"
	pb "github.com/hyperchain/go-hpc-rbft/rbftpb"
	"github.com/hyperchain/go-hpc-rbft/types"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

//******************************************************************************************************************
// some tools for unit tests
//******************************************************************************************************************
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

func execute(t *testing.T, rbfts []*rbftImpl, nodes []*testNode, tx *protos.Transaction, checkpoint bool) map[pb.Type][]*pb.ConsensusMessage {
	return executeExceptN(t, rbfts, nodes, tx, checkpoint, len(rbfts))
}

func executeExceptPrimary(t *testing.T, rbfts []*rbftImpl, nodes []*testNode, tx *protos.Transaction, checkpoint bool) map[pb.Type][]*pb.ConsensusMessage {
	return executeExceptN(t, rbfts, nodes, tx, checkpoint, 0)
}

func executeExceptN(t *testing.T, rbfts []*rbftImpl, nodes []*testNode, tx *protos.Transaction, checkpoint bool, notExec int) map[pb.Type][]*pb.ConsensusMessage {

	retMessages := make(map[pb.Type][]*pb.ConsensusMessage)

	rbfts[0].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)
	rbfts[1].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)
	rbfts[2].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)
	rbfts[3].batchMgr.requestPool.AddNewRequests([]*protos.Transaction{tx}, false, true)

	batchTimerEvent := &LocalEvent{
		Service:   CoreRbftService,
		EventType: CoreBatchTimerEvent,
	}
	rbfts[0].processEvent(batchTimerEvent)

	preprepMsg := nodes[0].broadcastMessageCache
	assert.Equal(t, pb.Type_PRE_PREPARE, preprepMsg.Type)
	retMessages[pb.Type_PRE_PREPARE] = []*pb.ConsensusMessage{preprepMsg}

	prepMsg := make([]*pb.ConsensusMessage, len(rbfts))
	commitMsg := make([]*pb.ConsensusMessage, len(rbfts))
	checkpointMsg := make([]*pb.ConsensusMessage, len(rbfts))

	for index := range rbfts {
		if index == 0 || index == notExec {
			continue
		}
		rbfts[index].processEvent(preprepMsg)
		prepMsg[index] = nodes[index].broadcastMessageCache
		assert.Equal(t, pb.Type_PREPARE, prepMsg[index].Type)
	}
	// for pre-prepare message
	// a primary won't process a pre-prepare, so that the prepMsg[0] will always be empty
	assert.Equal(t, (*pb.ConsensusMessage)(nil), prepMsg[0])
	retMessages[pb.Type_PREPARE] = prepMsg

	for index := range rbfts {
		if index == notExec {
			continue
		}
		for j := range prepMsg {
			if j == 0 || j == notExec {
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

// NewRawLoggerFile create log file for local cluster tests
func FrameworkNewRawLoggerFile(hostname string) *fancylogger.Logger {
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

// NewRawLoggerFile create log file for local cluster tests
func FrameworkNewRawLogger() *fancylogger.Logger {
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
	case *pb.FetchCheckpoint:
		eventType = pb.Type_FETCH_CHECKPOINT
		payload, err = proto.Marshal(et)
	case *pb.NewView:
		eventType = pb.Type_NEW_VIEW
		payload, err = proto.Marshal(et)
	case *pb.FetchRequestBatch:
		eventType = pb.Type_FETCH_REQUEST_BATCH
		payload, err = proto.Marshal(et)
	case *pb.SendRequestBatch:
		eventType = pb.Type_SEND_REQUEST_BATCH
		payload, err = proto.Marshal(et)
	case *pb.RecoveryFetchPQC:
		eventType = pb.Type_RECOVERY_FETCH_QPC
		payload, err = proto.Marshal(et)
	case *pb.RecoveryReturnPQC:
		eventType = pb.Type_RECOVERY_RETURN_QPC
		payload, err = proto.Marshal(et)
	case *pb.FetchMissingRequests:
		eventType = pb.Type_FETCH_MISSING_REQUESTS
		payload, err = proto.Marshal(et)
	case *pb.SendMissingRequests:
		eventType = pb.Type_SEND_MISSING_REQUESTS
		payload, err = proto.Marshal(et)
	case *pb.SyncState:
		eventType = pb.Type_SYNC_STATE
		payload, err = proto.Marshal(et)
	case *pb.SyncStateResponse:
		eventType = pb.Type_SYNC_STATE_RESPONSE
		payload, err = proto.Marshal(et)
	case *pb.Notification:
		eventType = pb.Type_NOTIFICATION
		payload, err = proto.Marshal(et)
	case *pb.NotificationResponse:
		eventType = pb.Type_NOTIFICATION_RESPONSE
		payload, err = proto.Marshal(et)
	case *pb.RequestSet:
		eventType = pb.Type_REQUEST_SET
		payload, err = proto.Marshal(et)
	case *pb.SignedCheckpoint:
		eventType = pb.Type_SIGNED_CHECKPOINT
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
		Epoch:   rbft.epoch,
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
