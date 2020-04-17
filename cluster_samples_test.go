package rbft

import (
	"github.com/ultramesh/flato-event/inner/protos"
	"strconv"
	"testing"
	"time"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/golang/mock/gomock"
	"github.com/magiconair/properties/assert"
)

func TestFunction_ViewChange(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// init cluster
	tf := newTestFramework(4)

	// start cluster
	tf.frameworkStart()

	preView := tf.TestNode[0].n.rbft.view
	tf.TestNode[int(preView+1)%4].n.rbft.sendViewChange()
	tf.TestNode[int(preView+2)%4].n.rbft.sendViewChange()
	tf.TestNode[int(preView+3)%4].n.rbft.sendViewChange()
	time.Sleep(2 * time.Second)

	var finishView uint64
	for _, node := range tf.TestNode {
		if !node.n.rbft.in(InViewChange) {
			finishView = node.n.rbft.view
			flag := finishView == preView
			assert.Equal(t, flag, false)
			break
		}
	}
	for _, node := range tf.TestNode {
		if !node.n.rbft.in(InViewChange) {
			flag := node.n.rbft.view == finishView
			assert.Equal(t, flag, true)
		}
	}
	tf.frameworkStop()
}

func TestFunction_FetchRequestBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// init cluster
	tf := newTestFramework(4)

	// start cluster
	tf.frameworkStart()
	// send tx
	tf.sendTx(uint64(1), uint64(0))
	time.Sleep(1 * time.Second)

	preView := tf.TestNode[0].n.rbft.view
	selectIndext := (int(preView) + 1) % 4
	var batchToDelete *pb.RequestBatch
	for key, val := range tf.TestNode[selectIndext].n.rbft.storeMgr.batchStore {
		batchToDelete = val
		delete(tf.TestNode[selectIndext].n.rbft.storeMgr.batchStore, key)
	}
	tf.TestNode[int(preView)%4].n.rbft.sendViewChange()
	time.Sleep(1 * time.Second)

	if batchToDelete != nil {
		batchAfterFetch := tf.TestNode[selectIndext].n.rbft.storeMgr.batchStore[batchToDelete.BatchHash]
		assert.Equal(t, batchToDelete.BatchHash, batchAfterFetch.BatchHash)
		assert.Equal(t, batchToDelete.RequestList, batchAfterFetch.RequestList)
		assert.Equal(t, batchToDelete.Timestamp, batchAfterFetch.Timestamp)
	}
	tf.frameworkStop()
}

func TestFunction_FetchMissingTxs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// init cluster
	tf := newTestFramework(4)

	// start cluster
	tf.frameworkStart()
	str := "tx" + strconv.FormatInt(0, 10)
	tx := &protos.Transaction{Value: []byte(str)}
	_ = tf.TestNode[0].n.rbft.batchMgr.requestPool.AddNewRequest(tx, false, true)
	_ = tf.TestNode[1].n.rbft.batchMgr.requestPool.AddNewRequest(tx, false, true)
	_ = tf.TestNode[2].n.rbft.batchMgr.requestPool.AddNewRequest(tx, false, true)
	_ = tf.TestNode[3].n.rbft.batchMgr.requestPool.AddNewRequest(tx, false, true)
	for _, node := range tf.TestNode {
		if !node.n.rbft.isPrimary(node.n.rbft.peerPool.localID) {
			node.n.rbft.batchMgr.requestPool.Reset()
		}
	}
	time.Sleep(1 * time.Second)

	getLastApplied := tf.TestNode[0].n.rbft.exec.lastExec
	for _, node := range tf.TestNode {
		assert.Equal(t, getLastApplied, node.n.rbft.exec.lastExec)
		assert.Equal(t, uint64(1), node.n.rbft.exec.lastExec)
	}
	tf.frameworkStop()
}

func TestFunction_UpdateNode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// init cluster
	tf := newTestFramework(4)

	// start cluster
	tf.frameworkStart()
	time.Sleep(1 * time.Second)

	// add node 5
	tf.frameworkAddNode("node5")
	time.Sleep(1 * time.Second)
	tf.TestNode[4].n.rbft.off(oneRoundOfEpochSync)
	tf.TestNode[4].n.rbft.off(InSyncState)
	tf.TestNode[4].n.rbft.tryEpochSync()
	time.Sleep(2 * time.Second)

	for i := uint64(4); i < uint64(13); i++ {
		tf.sendTx(i, uint64(1))
	}
	for _, node := range tf.TestNode {
		if !node.n.rbft.in(InConfChange) {
			assert.Equal(t, node.n.rbft.N, 5)
		}
	}
	tf.frameworkStop()
}

//
//func TestFunction_Init(t *testing.T) {
//	ctrl := gomock.NewController(t)
//	defer ctrl.Finish()
//
//	// init cluster
//	tf := newTestFramework(4)
//
//	// start cluster
//	tf.frameworkStart()
//	time.Sleep(1 * time.Second)
//
//	// init ctx
//	tf.sendTx(uint64(1), uint64(0))
//	time.Sleep(1 * time.Second)
//	tf.sendTx(uint64(2), uint64(0))
//	time.Sleep(1 * time.Second)
//	tf.sendInitCtx()
//	time.Sleep(1 * time.Second)
//
//	// add node 5
//	tf.frameworkAddNode("node5")
//	time.Sleep(1 * time.Second)
//
//	tf.sendTx(uint64(3), uint64(1))
//
//	// add node 6
//	tf.frameworkAddNode("node6")
//	time.Sleep(1 * time.Second)
//
//	for i := uint64(4); i < uint64(13); i++ {
//		tf.sendTx(i, uint64(1))
//		time.Sleep(1 * time.Second)
//	}
//
//	tf.frameworkDelNode("node3")
//	time.Sleep(1 * time.Second)
//
//	for i := uint64(20); i < uint64(24); i++ {
//		tf.sendTx(i, uint64(1))
//		time.Sleep(1 * time.Second)
//	}
//
//	// add node 7
//	tf.frameworkAddNode("node7")
//	time.Sleep(1 * time.Second)
//
//	for i := uint64(27); i < uint64(40); i++ {
//		tf.sendTx(i, uint64(1))
//		time.Sleep(1 * time.Second)
//	}
//
//	tf.frameworkDelNode("node5")
//	time.Sleep(1 * time.Second)
//
//	tf.frameworkDelNode("node4")
//	time.Sleep(1 * time.Second)
//
//	//tf.TestNode[0].n.rbft.sendViewChange()
//	tf.TestNode[1].n.rbft.sendViewChange()
//	tf.TestNode[4].n.rbft.sendViewChange()
//	tf.TestNode[6].n.rbft.sendViewChange()
//
//	time.Sleep(12 * time.Second)
//
//	// Stop cluster
//	tf.frameworkStop()
//}
