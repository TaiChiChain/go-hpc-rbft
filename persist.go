// Copyright 2016-2017 Hyperchain Corp.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rbft

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/types"
)

// persistQSet persists marshaled pre-prepare message to database
func (rbft *rbftImpl[T, Constraint]) persistQSet(preprep *consensus.PrePrepare) {
	if preprep == nil {
		rbft.logger.Debugf("Replica %d ignore nil prePrepare", rbft.peerMgr.selfID)
		return
	}

	raw, err := proto.Marshal(preprep)
	if err != nil {
		rbft.logger.Warningf("Replica %d could not persist qset: %s", rbft.peerMgr.selfID, err)
		return
	}
	key := fmt.Sprintf("qset.%d.%d.%s", preprep.View, preprep.SequenceNumber, preprep.BatchDigest)
	err = rbft.storage.StoreState(key, raw)
	if err != nil {
		rbft.logger.Errorf("Persist qset failed with err: %s ", err.Error())
	}
}

// persistPSet persists marshaled prepare messages in the cert with the given msgID(v,n,d) to database
func (rbft *rbftImpl[T, Constraint]) persistPSet(v uint64, n uint64, d string) {
	cert := rbft.storeMgr.getCert(v, n, d)
	set := make([]*consensus.Prepare, 0)
	pset := &consensus.Pset{Set: set}
	for p := range cert.prepare {
		tmp := p
		pset.Set = append(pset.Set, &tmp)
	}

	raw, err := proto.Marshal(pset)
	if err != nil {
		rbft.logger.Warningf("Replica %d could not persist pset: %s", rbft.peerMgr.selfID, err)
		return
	}
	key := fmt.Sprintf("pset.%d.%d.%s", v, n, d)
	err = rbft.storage.StoreState(key, raw)
	if err != nil {
		rbft.logger.Errorf("Persist pset failed with err: %s ", err.Error())
	}
}

// persistCSet persists marshaled commit messages in the cert with the given msgID(v,n,d) to database
func (rbft *rbftImpl[T, Constraint]) persistCSet(v uint64, n uint64, d string) {
	cert := rbft.storeMgr.getCert(v, n, d)
	set := make([]*consensus.Commit, 0)
	cset := &consensus.Cset{Set: set}
	for c := range cert.commit {
		tmp := c
		cset.Set = append(cset.Set, &tmp)
	}

	raw, err := proto.Marshal(cset)
	if err != nil {
		rbft.logger.Warningf("Replica %d could not persist cset: %s", rbft.peerMgr.selfID, err)
		return
	}
	key := fmt.Sprintf("cset.%d.%d.%s", v, n, d)
	err = rbft.storage.StoreState(key, raw)
	if err != nil {
		rbft.logger.Errorf("Persist cset failed with err: %s ", err.Error())
	}
}

// persistDelQSet deletes marshaled pre-prepare message with the given key from database
func (rbft *rbftImpl[T, Constraint]) persistDelQSet(v uint64, n uint64, d string) {
	qset := fmt.Sprintf("qset.%d.%d.%s", v, n, d)
	_ = rbft.storage.DelState(qset)
}

// persistDelPSet deletes marshaled prepare messages with the given key from database
func (rbft *rbftImpl[T, Constraint]) persistDelPSet(v uint64, n uint64, d string) {
	pset := fmt.Sprintf("pset.%d.%d.%s", v, n, d)
	_ = rbft.storage.DelState(pset)
}

// persistDelCSet deletes marshaled commit messages with the given key from database
func (rbft *rbftImpl[T, Constraint]) persistDelCSet(v uint64, n uint64, d string) {
	cset := fmt.Sprintf("cset.%d.%d.%s", v, n, d)
	_ = rbft.storage.DelState(cset)
}

// persistDelQPCSet deletes marshaled pre-prepare,prepare,commit messages with the given key from database
func (rbft *rbftImpl[T, Constraint]) persistDelQPCSet(v uint64, n uint64, d string) {
	rbft.persistDelQSet(v, n, d)
	rbft.persistDelPSet(v, n, d)
	rbft.persistDelCSet(v, n, d)
}

// restoreQSet restores pre-prepare messages from database, which, keyed by msgID
func (rbft *rbftImpl[T, Constraint]) restoreQSet() (map[msgID]*consensus.PrePrepare, error) {
	qset := make(map[msgID]*consensus.PrePrepare)
	payload, err := rbft.storage.ReadStateSet("qset.")
	if err == nil {
		for key, set := range payload {
			var v, n uint64
			var d string
			v, n, d, err = rbft.parseQPCKey(key, "qset")
			if err != nil {
				rbft.logger.Warningf("Replica %d could not restore qset key %s, err: %s", rbft.peerMgr.selfID, key, err)
			} else {
				preprep := &consensus.PrePrepare{}
				err = proto.Unmarshal(set, preprep)
				if err == nil {
					idx := msgID{v: v, n: n, d: d}
					qset[idx] = preprep
				} else {
					rbft.logger.Warningf("Could not restore prePrepare %v, err: %v", set, err)
				}
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d could not restore qset: %s", rbft.peerMgr.selfID, err)
	}

	return qset, err
}

// restorePSet restores prepare messages from database, which, keyed by msgID
func (rbft *rbftImpl[T, Constraint]) restorePSet() (map[msgID]*consensus.Pset, error) {
	pset := make(map[msgID]*consensus.Pset)
	payload, err := rbft.storage.ReadStateSet("pset.")
	if err == nil {
		for key, set := range payload {
			var v, n uint64
			var d string
			v, n, d, err = rbft.parseQPCKey(key, "pset")
			if err != nil {
				rbft.logger.Warningf("Replica %d could not restore pset key %s, err: %s", rbft.peerMgr.selfID, key, err)
			} else {
				prepares := &consensus.Pset{}
				err = proto.Unmarshal(set, prepares)
				if err == nil {
					idx := msgID{v: v, n: n, d: d}
					pset[idx] = prepares
				} else {
					rbft.logger.Warningf("Replica %d could not restore prepares %v", rbft.peerMgr.selfID, set)
				}
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d could not restore pset: %s", rbft.peerMgr.selfID, err)
	}

	return pset, err
}

// restoreCSet restores commit messages from database, which, keyed by msgID
func (rbft *rbftImpl[T, Constraint]) restoreCSet() (map[msgID]*consensus.Cset, error) {
	cset := make(map[msgID]*consensus.Cset)

	payload, err := rbft.storage.ReadStateSet("cset.")
	if err == nil {
		for key, set := range payload {
			var v, n uint64
			var d string
			v, n, d, err = rbft.parseQPCKey(key, "cset")
			if err != nil {
				rbft.logger.Warningf("Replica %d could not restore pset key %s, err: %s", rbft.peerMgr.selfID, key, err)
			} else {
				commits := &consensus.Cset{}
				err = proto.Unmarshal(set, commits)
				if err == nil {
					idx := msgID{v: v, n: n, d: d}
					cset[idx] = commits
				} else {
					rbft.logger.Warningf("Replica %d could not restore commits %v", rbft.peerMgr.selfID, set)
				}
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d could not restore cset: %s", rbft.peerMgr.selfID, err)
	}

	return cset, err
}

// persistQList persists marshaled qList into DB before vc.
func (rbft *rbftImpl[T, Constraint]) persistQList(ql map[qidx]*consensus.Vc_PQ) {
	for idx, q := range ql {
		raw, err := proto.Marshal(q)
		if err != nil {
			rbft.logger.Warningf("Replica %d could not persist qlist with index %+v, error : %s", rbft.peerMgr.selfID, idx, err)
			continue
		}
		key := fmt.Sprintf("qlist.%d.%s", idx.n, idx.d)
		err = rbft.external.StoreState(key, raw)
		if err != nil {
			rbft.logger.Errorf("Persist qlist failed with err: %s ", err)
		}
	}
}

// persistPList persists marshaled pList into DB before vc.
func (rbft *rbftImpl[T, Constraint]) persistPList(pl map[uint64]*consensus.Vc_PQ) {
	for idx, p := range pl {
		raw, err := proto.Marshal(p)
		if err != nil {
			rbft.logger.Warningf("Replica %d could not persist plist with index %+v, error : %s", rbft.peerMgr.selfID, idx, err)
			continue
		}
		key := fmt.Sprintf("plist.%d", idx)
		err = rbft.external.StoreState(key, raw)
		if err != nil {
			rbft.logger.Errorf("Persist plist failed with err: %s ", err)
		}
	}
}

// persistDelQPList deletes all qList and pList stored in DB after finish vc.
func (rbft *rbftImpl[T, Constraint]) persistDelQPList() {
	qIndex, err := rbft.external.ReadStateSet("qlist.")
	if err != nil {
		rbft.logger.Debug("not found qList to delete")
	} else {
		for k := range qIndex {
			_ = rbft.external.DelState(k)
		}
	}

	pIndex, err := rbft.external.ReadStateSet("plist.")
	if err != nil {
		rbft.logger.Debug("not found pList to delete")
	} else {
		for k := range pIndex {
			_ = rbft.external.DelState(k)
		}
	}
}

// restoreQList restores qList from DB, which, keyed by qidx
func (rbft *rbftImpl[T, Constraint]) restoreQList() (map[qidx]*consensus.Vc_PQ, error) {
	qList := make(map[qidx]*consensus.Vc_PQ)
	payload, err := rbft.external.ReadStateSet("qlist.")
	if err == nil {
		for key, value := range payload {
			var n int
			var d string
			splitKeys := strings.Split(key, ".")
			if len(splitKeys) != 3 {
				rbft.logger.Warningf("Replica %d could not restore key %s", rbft.peerMgr.selfID, key)
				return nil, errors.New("incorrect format")
			}

			if splitKeys[0] != "qlist" {
				rbft.logger.Errorf("Replica %d finds error key prefix when restore qList using %s", rbft.peerMgr.selfID, key)
				return nil, errors.New("incorrect prefix")
			}

			n, err = strconv.Atoi(splitKeys[1])
			if err != nil {
				rbft.logger.Errorf("Replica %d could not parse key %s to int", rbft.peerMgr.selfID, splitKeys[1])
				return nil, errors.New("parse failed")
			}

			d = splitKeys[2]

			q := &consensus.Vc_PQ{}
			err = proto.Unmarshal(value, q)
			if err == nil {
				rbft.logger.Debugf("Replica %d restore qList %+v", rbft.peerMgr.selfID, q)
				idx := qidx{d: d, n: uint64(n)}
				qList[idx] = q
			} else {
				rbft.logger.Warningf("Replica %d could not restore qList %v", rbft.peerMgr.selfID, value)
			}
		}
	} else {
		rbft.logger.Debugf("Replica %d could not restore qList: %s", rbft.peerMgr.selfID, err)
	}
	return qList, err
}

// restorePList restores pList from DB
func (rbft *rbftImpl[T, Constraint]) restorePList() (map[uint64]*consensus.Vc_PQ, error) {
	pList := make(map[uint64]*consensus.Vc_PQ)
	payload, err := rbft.external.ReadStateSet("plist.")
	if err == nil {
		for key, value := range payload {
			var n int
			splitKeys := strings.Split(key, ".")
			if len(splitKeys) != 2 {
				rbft.logger.Warningf("Replica %d could not restore key %s", rbft.peerMgr.selfID, key)
				return nil, errors.New("incorrect format")
			}

			if splitKeys[0] != "plist" {
				rbft.logger.Errorf("Replica %d finds error key prefix when restore pList using %s", rbft.peerMgr.selfID, key)
				return nil, errors.New("incorrect prefix")
			}

			n, err = strconv.Atoi(splitKeys[1])
			if err != nil {
				rbft.logger.Errorf("Replica %d could not parse key %s to int", rbft.peerMgr.selfID, splitKeys[1])
				return nil, errors.New("parse failed")
			}

			p := &consensus.Vc_PQ{}
			err = proto.Unmarshal(value, p)
			if err == nil {
				rbft.logger.Debugf("Replica %d restore pList %+v", rbft.peerMgr.selfID, p)
				pList[uint64(n)] = p
			} else {
				rbft.logger.Warningf("Replica %d could not restore pList %v", rbft.peerMgr.selfID, value)
			}
		}
	} else {
		rbft.logger.Debugf("Replica %d could not restore pList: %s", rbft.peerMgr.selfID, err)
	}
	return pList, err
}

// restoreCert restores pre-prepares,prepares,commits from database and remove the messages with seqNo>lastExec
func (rbft *rbftImpl[T, Constraint]) restoreCert() {
	qset, _ := rbft.restoreQSet()
	for idx, q := range qset {
		if idx.n > rbft.exec.lastExec {
			rbft.logger.Debugf("Replica %d restore qSet with seqNo %d > lastExec %d", rbft.peerMgr.selfID, idx.n, rbft.exec.lastExec)
		}
		cert := rbft.storeMgr.getCert(idx.v, idx.n, idx.d)
		cert.prePrepare = q
		cert.prePrepareCtx = context.TODO()
		batch, ok := rbft.storeMgr.batchStore[idx.d]
		// set isConfig if found.
		if ok {
			cert.isConfig = isConfigBatch(batch.SeqNo, rbft.chainConfig.EpochInfo)
		}
	}

	pset, _ := rbft.restorePSet()
	for idx, prepares := range pset {
		if idx.n > rbft.exec.lastExec {
			rbft.logger.Debugf("Replica %d restore pSet with seqNo %d > lastExec %d", rbft.peerMgr.selfID, idx.n, rbft.exec.lastExec)
		}
		cert := rbft.storeMgr.getCert(idx.v, idx.n, idx.d)
		for _, p := range prepares.Set {
			cert.prepare[*p] = true
			if p.ReplicaId == rbft.peerMgr.selfID && idx.n <= rbft.exec.lastExec {
				cert.sentPrepare = true
			}
		}
	}

	cset, _ := rbft.restoreCSet()
	for idx, commits := range cset {
		if idx.n > rbft.exec.lastExec {
			rbft.logger.Debugf("Replica %d restore cSet with seqNo %d > lastExec %d", rbft.peerMgr.selfID, idx.n, rbft.exec.lastExec)
		}
		cert := rbft.storeMgr.getCert(idx.v, idx.n, idx.d)
		for _, c := range commits.Set {
			cert.commit[*c] = true
			if c.ReplicaId == rbft.peerMgr.selfID && idx.n <= rbft.exec.lastExec {
				cert.sentCommit = true
			}
		}
	}
	for idx, cert := range rbft.storeMgr.certStore {
		if idx.n <= rbft.exec.lastExec {
			cert.sentExecute = true
		}
	}

	// restore qpList if any.
	qList, err := rbft.restoreQList()
	if err == nil {
		rbft.vcMgr.qlist = qList
	}

	pList, err := rbft.restorePList()
	if err == nil {
		rbft.vcMgr.plist = pList
	}
}

// persistBatch persists one marshaled tx batch with the given digest to database
func (rbft *rbftImpl[T, Constraint]) persistBatch(digest string) {
	batch := rbft.storeMgr.batchStore[digest]
	batchPacked, err := batch.Marshal()
	if err != nil {
		rbft.logger.Warningf("Replica %d could not persist request batch %s: %s", rbft.peerMgr.selfID, digest, err)
		return
	}
	start := time.Now()
	err = rbft.storage.StoreState("batch."+digest, batchPacked)
	if err != nil {
		rbft.logger.Errorf("Persist batch failed with err: %s ", err)
	}
	duration := time.Since(start).Seconds()
	rbft.metrics.batchPersistDuration.Observe(duration)
}

// persistDelBatch removes one marshaled tx batch with the given digest from database
func (rbft *rbftImpl[T, Constraint]) persistDelBatch(digest string) {
	_ = rbft.storage.DelState("batch." + digest)
}

// persistCheckpoint persists checkpoint to database, which, key contains the seqNo of checkpoint, value is the
// checkpoint ID
func (rbft *rbftImpl[T, Constraint]) persistCheckpoint(seqNo uint64, id []byte) {
	key := fmt.Sprintf("chkpt.%d", seqNo)
	err := rbft.storage.StoreState(key, id)
	if err != nil {
		rbft.logger.Errorf("Persist chkpt failed with err: %s ", err)
	}
}

// persistDelCheckpoint deletes checkpoint with the given seqNo from database
func (rbft *rbftImpl[T, Constraint]) persistDelCheckpoint(seqNo uint64) {
	key := fmt.Sprintf("chkpt.%d", seqNo)
	_ = rbft.storage.DelState(key)
}

func (rbft *rbftImpl[T, Constraint]) persistH(seqNo uint64) {
	err := rbft.storage.StoreState("rbft.h", []byte(strconv.FormatUint(seqNo, 10)))
	if err != nil {
		rbft.logger.Errorf("Persist h failed with err: %s ", err)
	}
}

// persistNewView persists current view to database
func (rbft *rbftImpl[T, Constraint]) persistNewView(nv *consensus.NewView) {
	key := "new-view"
	raw, err := proto.Marshal(nv)
	if err != nil {
		rbft.logger.Warningf("Replica %d could not persist NewView, error : %s", rbft.peerMgr.selfID, err)
		rbft.stopNamespace()
		return
	}
	err = rbft.storage.StoreState(key, raw)
	if err != nil {
		rbft.logger.Errorf("Persist NewView failed with err: %s ", err)
	}

	rbft.setView(nv.View)
	rbft.vcMgr.latestNewView = nv
	rbft.vcMgr.latestQuorumViewChange = nil
}

// restoreView restores current view from database and then re-construct certStore
func (rbft *rbftImpl[T, Constraint]) restoreView() {
	raw, err := rbft.storage.ReadState("new-view")
	if err == nil {
		nv := &consensus.NewView{}
		err = proto.Unmarshal(raw, nv)
		if err == nil {
			rbft.logger.Debugf("Replica %d restore view %d", rbft.peerMgr.selfID, nv.View)
			rbft.vcMgr.latestNewView = nv
			rbft.setView(nv.View)
			rbft.logger.Noticef("========= restore view %d =======", rbft.chainConfig.View)
			return
		}
	}
	rbft.logger.Warningf("Replica %d could not restore view: %s, set to 0", rbft.peerMgr.selfID, err)
	// initial view 0 in new epoch.
	rbft.vcMgr.latestNewView = initialNewView
	rbft.setView(0)
}

// restoreBatchStore restores tx batches from database
func (rbft *rbftImpl[T, Constraint]) restoreBatchStore() {
	payload, err := rbft.storage.ReadStateSet("batch.")
	if err == nil {
		for key, set := range payload {
			var digest string
			if _, err = fmt.Sscanf(key, "batch.%s", &digest); err != nil {
				rbft.logger.Warningf("Replica %d could not restore pset key %s", rbft.peerMgr.selfID, key)
			} else {
				batch := &RequestBatch[T, Constraint]{}
				err = batch.Unmarshal(set)
				if err == nil {
					rbft.logger.Debugf("Replica %d restore batch %s", rbft.peerMgr.selfID, digest)
					rbft.storeMgr.batchStore[digest] = batch
					rbft.metrics.batchesGauge.Add(float64(1))
				} else {
					rbft.logger.Warningf("Replica %d could not unmarshal batch key %s for error: %v", rbft.peerMgr.selfID, key, err)
				}
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d could not restore batch: %v", rbft.peerMgr.selfID, err)
	}
}

func (rbft *rbftImpl[T, Constraint]) restoreEpochInfo() {
	e, err := rbft.external.GetCurrentEpochInfo()
	if err != nil {
		rbft.logger.Warningf("Replica %d failed to get current epoch from ledger: %v, will use genesis epoch info", rbft.peerMgr.selfID, err)
		rbft.chainConfig.EpochInfo = rbft.config.GenesisEpochInfo
	} else {
		rbft.chainConfig.EpochInfo = e
	}
	rbft.epochMgr.epoch = rbft.chainConfig.EpochInfo.Epoch
}

// It is application's responsibility to ensure data compatibility, so RBFT core need only trust and restore
// consensus data from consensus DB.
// restoreState restores lastExec, certStore, view, transaction batches, checkpoints, h and other related
// params from database
func (rbft *rbftImpl[T, Constraint]) restoreState() error {
	rbft.restoreEpochInfo()
	rbft.chainConfig.updateDerivedData()
	rbft.batchMgr.setSeqNo(rbft.exec.lastExec)
	if err := rbft.peerMgr.updateRoutingTable(rbft.chainConfig); err != nil {
		return err
	}
	rbft.restoreView()
	rbft.restoreBatchStore()
	rbft.restoreCert()

	// mock an initial checkpoint.
	state := &types.ServiceState{
		MetaState: &types.MetaState{
			Height: rbft.config.GenesisEpochInfo.StartBlock,
			Digest: rbft.config.GenesisBlockDigest,
		},
		Epoch: rbft.config.GenesisEpochInfo.Epoch,
	}
	mockCheckpoint, gErr := rbft.generateSignedCheckpoint(state, false)
	if gErr != nil {
		rbft.metrics.unregisterMetrics()
		rbft.metrics = nil
		return gErr
	}
	rbft.storeMgr.localCheckpoints[state.MetaState.Height] = mockCheckpoint

	chkpts, err := rbft.storage.ReadStateSet("chkpt.")
	if err == nil {
		var maxCheckpointSeqNo uint64
		for key, id := range chkpts {
			var seqNo uint64
			if _, err = fmt.Sscanf(key, "chkpt.%d", &seqNo); err != nil {
				rbft.logger.Warningf("Replica %d could not restore checkpoint key %s", rbft.peerMgr.selfID, key)
			} else {
				digest := string(id)
				rbft.logger.Debugf("Replica %d found checkpoint %s for seqNo %d", rbft.peerMgr.selfID, digest, seqNo)
				state := &types.ServiceState{
					MetaState: &types.MetaState{Height: seqNo, Digest: digest},
					Epoch:     rbft.chainConfig.EpochInfo.Epoch,
				}
				signedC, gErr := rbft.generateSignedCheckpoint(state, isConfigBatch(seqNo, rbft.chainConfig.EpochInfo))
				if gErr != nil {
					return gErr
				}
				rbft.storeMgr.saveCheckpoint(seqNo, signedC)
				if seqNo > maxCheckpointSeqNo {
					rbft.chainConfig.LastCheckpointExecBlockHash = digest
					maxCheckpointSeqNo = seqNo
				}
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d could not restore checkpoints: %s", rbft.peerMgr.selfID, err)
	}
	rbft.chainConfig.updatePrimaryID()

	hstr, rErr := rbft.storage.ReadState("rbft.h")
	if rErr != nil {
		rbft.logger.Warningf("Replica %d could not restore h: %s", rbft.peerMgr.selfID, rErr)
	} else {
		h, err := strconv.ParseUint(string(hstr), 10, 64)
		if err != nil {
			rbft.logger.Warningf("transfer rbft.h from string to uint64 failed with err: %s", err)
			return err
		}
		rbft.moveWatermarks(h, false)
	}

	rbft.logger.Infof("Replica %d restored state: epoch: %d, view: %d, seqNo: %d, "+
		"reqBatches: %d, localCheckpoints: %d", rbft.peerMgr.selfID, rbft.chainConfig.EpochInfo.Epoch, rbft.chainConfig.View, rbft.exec.lastExec,
		len(rbft.storeMgr.batchStore), len(rbft.storeMgr.localCheckpoints))

	return nil
}

// parseQPCKey helps parse view, seqNo, digest from given key with prefix.
func (rbft *rbftImpl[T, Constraint]) parseQPCKey(key, prefix string) (uint64, uint64, string, error) {
	var (
		v, n int
		d    string
		err  error
	)

	splitKeys := strings.Split(key, ".")
	if len(splitKeys) != 4 {
		rbft.logger.Warningf("Replica %d could not restore key %s with prefix %s", rbft.peerMgr.selfID, key, prefix)
		return 0, 0, "", errors.New("incorrect format")
	}

	if splitKeys[0] != prefix {
		rbft.logger.Errorf("Replica %d finds error key prefix when restore %s using %s", rbft.peerMgr.selfID, prefix, key)
		return 0, 0, "", errors.New("incorrect prefix")
	}

	v, err = strconv.Atoi(splitKeys[1])
	if err != nil {
		rbft.logger.Errorf("Replica %d could not parse key %s to int", rbft.peerMgr.selfID, splitKeys[1])
		return 0, 0, "", errors.New("parse failed")
	}

	n, err = strconv.Atoi(splitKeys[2])
	if err != nil {
		rbft.logger.Errorf("Replica %d could not parse key %s to int", rbft.peerMgr.selfID, splitKeys[2])
		return 0, 0, "", errors.New("parse failed")
	}

	d = splitKeys[3]
	return uint64(v), uint64(n), d, nil
}
