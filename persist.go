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
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "github.com/ultramesh/flato-rbft/rbftpb"

	"github.com/gogo/protobuf/proto"
)

// persistQSet persists marshaled pre-prepare message to database
func (rbft *rbftImpl) persistQSet(preprep *pb.PrePrepare) {
	if preprep == nil {
		rbft.logger.Debugf("Replica %d ignore nil prePrepare", rbft.peerPool.ID)
		return
	}

	raw, err := proto.Marshal(preprep)
	if err != nil {
		rbft.logger.Warningf("Replica %d could not persist qset: %s", rbft.peerPool.ID, err)
		return
	}
	key := fmt.Sprintf("qset.%d.%d.%s", preprep.View, preprep.SequenceNumber, preprep.BatchDigest)
	err = rbft.storage.StoreState(key, raw)
	if err != nil {
		rbft.logger.Errorf("Persist qset failed with err: %s ", err.Error())
	}
}

// persistPSet persists marshaled prepare messages in the cert with the given msgID(v,n,d) to database
func (rbft *rbftImpl) persistPSet(v uint64, n uint64, d string) {
	cert := rbft.storeMgr.getCert(v, n, d)
	set := make([]*pb.Prepare, 0)
	pset := &pb.Pset{Set: set}
	for p := range cert.prepare {
		tmp := p
		pset.Set = append(pset.Set, &tmp)
	}

	raw, err := proto.Marshal(pset)
	if err != nil {
		rbft.logger.Warningf("Replica %d could not persist pset: %s", rbft.peerPool.ID, err)
		return
	}
	key := fmt.Sprintf("pset.%d.%d.%s", v, n, d)
	err = rbft.storage.StoreState(key, raw)
	if err != nil {
		rbft.logger.Errorf("Persist pset failed with err: %s ", err.Error())
	}
}

// persistCSet persists marshaled commit messages in the cert with the given msgID(v,n,d) to database
func (rbft *rbftImpl) persistCSet(v uint64, n uint64, d string) {
	cert := rbft.storeMgr.getCert(v, n, d)
	set := make([]*pb.Commit, 0)
	cset := &pb.Cset{Set: set}
	for c := range cert.commit {
		tmp := c
		cset.Set = append(cset.Set, &tmp)
	}

	raw, err := proto.Marshal(cset)
	if err != nil {
		rbft.logger.Warningf("Replica %d could not persist cset: %s", rbft.peerPool.ID, err)
		return
	}
	key := fmt.Sprintf("cset.%d.%d.%s", v, n, d)
	err = rbft.storage.StoreState(key, raw)
	if err != nil {
		rbft.logger.Errorf("Persist cset failed with err: %s ", err.Error())
	}
}

// persistDelQSet deletes marshaled pre-prepare message with the given key from database
func (rbft *rbftImpl) persistDelQSet(v uint64, n uint64, d string) {
	qset := fmt.Sprintf("qset.%d.%d.%s", v, n, d)
	_ = rbft.storage.DelState(qset)
}

// persistDelPSet deletes marshaled prepare messages with the given key from database
func (rbft *rbftImpl) persistDelPSet(v uint64, n uint64, d string) {
	pset := fmt.Sprintf("pset.%d.%d.%s", v, n, d)
	_ = rbft.storage.DelState(pset)
}

// persistDelCSet deletes marshaled commit messages with the given key from database
func (rbft *rbftImpl) persistDelCSet(v uint64, n uint64, d string) {
	cset := fmt.Sprintf("cset.%d.%d.%s", v, n, d)
	_ = rbft.storage.DelState(cset)
}

// persistDelQPCSet deletes marshaled pre-prepare,prepare,commit messages with the given key from database
func (rbft *rbftImpl) persistDelQPCSet(v uint64, n uint64, d string) {
	rbft.persistDelQSet(v, n, d)
	rbft.persistDelPSet(v, n, d)
	rbft.persistDelCSet(v, n, d)
}

// restoreQSet restores pre-prepare messages from database, which, keyed by msgID
func (rbft *rbftImpl) restoreQSet() (map[msgID]*pb.PrePrepare, error) {
	qset := make(map[msgID]*pb.PrePrepare)
	payload, err := rbft.storage.ReadStateSet("qset.")
	if err == nil {
		for key, set := range payload {
			var v, n uint64
			var d string
			v, n, d, err = rbft.parseQPCKey(key, "qset")
			if err != nil {
				rbft.logger.Warningf("Replica %d could not restore qset key %s, err: %s", rbft.peerPool.ID, key, err)
			} else {
				preprep := &pb.PrePrepare{}
				err = proto.Unmarshal(set, preprep)
				if err == nil {
					idx := msgID{v, n, d}
					qset[idx] = preprep
				} else {
					rbft.logger.Warningf("Could not restore prePrepare %v, err: %v", set, err)
				}
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d could not restore qset: %s", rbft.peerPool.ID, err)
	}

	return qset, err
}

// restorePSet restores prepare messages from database, which, keyed by msgID
func (rbft *rbftImpl) restorePSet() (map[msgID]*pb.Pset, error) {
	pset := make(map[msgID]*pb.Pset)
	payload, err := rbft.storage.ReadStateSet("pset.")
	if err == nil {
		for key, set := range payload {
			var v, n uint64
			var d string
			v, n, d, err = rbft.parseQPCKey(key, "pset")
			if err != nil {
				rbft.logger.Warningf("Replica %d could not restore pset key %s, err: %s", rbft.peerPool.ID, key, err)
			} else {
				prepares := &pb.Pset{}
				err = proto.Unmarshal(set, prepares)
				if err == nil {
					idx := msgID{v, n, d}
					pset[idx] = prepares
				} else {
					rbft.logger.Warningf("Replica %d could not restore prepares %v", rbft.peerPool.ID, set)
				}
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d could not restore pset: %s", rbft.peerPool.ID, err)
	}

	return pset, err
}

// restoreCSet restores commit messages from database, which, keyed by msgID
func (rbft *rbftImpl) restoreCSet() (map[msgID]*pb.Cset, error) {
	cset := make(map[msgID]*pb.Cset)

	payload, err := rbft.storage.ReadStateSet("cset.")
	if err == nil {
		for key, set := range payload {
			var v, n uint64
			var d string
			v, n, d, err = rbft.parseQPCKey(key, "cset")
			if err != nil {
				rbft.logger.Warningf("Replica %d could not restore pset key %s, err: %s", rbft.peerPool.ID, key, err)
			} else {
				commits := &pb.Cset{}
				err = proto.Unmarshal(set, commits)
				if err == nil {
					idx := msgID{v, n, d}
					cset[idx] = commits
				} else {
					rbft.logger.Warningf("Replica %d could not restore commits %v", rbft.peerPool.ID, set)
				}
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d could not restore cset: %s", rbft.peerPool.ID, err)
	}

	return cset, err
}

// persistQList persists marshaled qList into DB before vc/recovery.
func (rbft *rbftImpl) persistQList(ql map[qidx]*pb.Vc_PQ) {
	for idx, q := range ql {
		raw, err := proto.Marshal(q)
		if err != nil {
			rbft.logger.Warningf("Replica %d could not persist qlist with index %+v, error : %s", rbft.peerPool.ID, idx, err)
			continue
		}
		key := fmt.Sprintf("qlist.%d.%s", idx.n, idx.d)
		err = rbft.external.StoreState(key, raw)
		if err != nil {
			rbft.logger.Errorf("Persist qlist failed with err: %s ", err)
		}
	}
}

// persistPList persists marshaled pList into DB before vc/recovery.
func (rbft *rbftImpl) persistPList(pl map[uint64]*pb.Vc_PQ) {
	for idx, p := range pl {
		raw, err := proto.Marshal(p)
		if err != nil {
			rbft.logger.Warningf("Replica %d could not persist plist with index %+v, error : %s", rbft.peerPool.ID, idx, err)
			continue
		}
		key := fmt.Sprintf("plist.%d", idx)
		err = rbft.external.StoreState(key, raw)
		if err != nil {
			rbft.logger.Errorf("Persist plist failed with err: %s ", err)
		}
	}
}

// persistDelQPList deletes all qList and pList stored in DB after finish vc/recovery.
func (rbft *rbftImpl) persistDelQPList() {
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
func (rbft *rbftImpl) restoreQList() (map[qidx]*pb.Vc_PQ, error) {
	qList := make(map[qidx]*pb.Vc_PQ)
	payload, err := rbft.external.ReadStateSet("qlist.")
	if err == nil {
		for key, value := range payload {
			var n int
			var d string
			splitKeys := strings.Split(key, ".")
			if len(splitKeys) != 3 {
				rbft.logger.Warningf("Replica %d could not restore key %s", rbft.peerPool.ID, key)
				return nil, errors.New("incorrect format")
			}

			if splitKeys[0] != "qlist" {
				rbft.logger.Errorf("Replica %d finds error key prefix when restore qList using %s", rbft.peerPool.ID, key)
				return nil, errors.New("incorrect prefix")
			}

			n, err = strconv.Atoi(splitKeys[1])
			if err != nil {
				rbft.logger.Errorf("Replica %d could not parse key %s to int", rbft.peerPool.ID, splitKeys[1])
				return nil, errors.New("parse failed")
			}

			d = splitKeys[2]

			q := &pb.Vc_PQ{}
			err = proto.Unmarshal(value, q)
			if err == nil {
				rbft.logger.Debugf("Replica %d restore qList %+v", rbft.peerPool.ID, q)
				idx := qidx{d, uint64(n)}
				qList[idx] = q
			} else {
				rbft.logger.Warningf("Replica %d could not restore qList %v", rbft.peerPool.ID, value)
			}
		}
	} else {
		rbft.logger.Debugf("Replica %d could not restore qList: %s", rbft.peerPool.ID, err)
	}
	return qList, err
}

// restorePList restores pList from DB
func (rbft *rbftImpl) restorePList() (map[uint64]*pb.Vc_PQ, error) {
	pList := make(map[uint64]*pb.Vc_PQ)
	payload, err := rbft.external.ReadStateSet("plist.")
	if err == nil {
		for key, value := range payload {
			var n int
			splitKeys := strings.Split(key, ".")
			if len(splitKeys) != 2 {
				rbft.logger.Warningf("Replica %d could not restore key %s", rbft.peerPool.ID, key)
				return nil, errors.New("incorrect format")
			}

			if splitKeys[0] != "plist" {
				rbft.logger.Errorf("Replica %d finds error key prefix when restore pList using %s", rbft.peerPool.ID, key)
				return nil, errors.New("incorrect prefix")
			}

			n, err = strconv.Atoi(splitKeys[1])
			if err != nil {
				rbft.logger.Errorf("Replica %d could not parse key %s to int", rbft.peerPool.ID, splitKeys[1])
				return nil, errors.New("parse failed")
			}

			p := &pb.Vc_PQ{}
			err = proto.Unmarshal(value, p)
			if err == nil {
				rbft.logger.Debugf("Replica %d restore pList %+v", rbft.peerPool.ID, p)
				pList[uint64(n)] = p
			} else {
				rbft.logger.Warningf("Replica %d could not restore pList %v", rbft.peerPool.ID, value)
			}
		}
	} else {
		rbft.logger.Debugf("Replica %d could not restore pList: %s", rbft.peerPool.ID, err)
	}
	return pList, err
}

// restoreCert restores pre-prepares,prepares,commits from database and remove the messages with seqNo>lastExec
func (rbft *rbftImpl) restoreCert() {
	var clean bool
	cleanCert, err := rbft.storage.ReadState("cleanCert")
	// delete this key immediately no matter this key exists or not to totally avoid
	// cleanCert again in next restart.
	_ = rbft.storage.DelState("cleanCert")
	// if we have stored key "cleanCert" with value "true" using dbcli, then we need to clean
	// cert with seqNo > lastExec.
	if err == nil && string(cleanCert) == "true" {
		clean = true
	}

	qset, _ := rbft.restoreQSet()
	for idx, q := range qset {
		if idx.n > rbft.exec.lastExec {
			if clean {
				rbft.logger.Debugf("Replica %d clean qSet with seqNo %d > lastExec %d", rbft.peerPool.ID, idx.n, rbft.exec.lastExec)
				rbft.persistDelQSet(idx.v, idx.n, idx.d)
				continue
			}
			rbft.logger.Debugf("Replica %d restore qSet with seqNo %d > lastExec %d", rbft.peerPool.ID, idx.n, rbft.exec.lastExec)
		}
		cert := rbft.storeMgr.getCert(idx.v, idx.n, idx.d)
		cert.prePrepare = q
	}

	pset, _ := rbft.restorePSet()
	for idx, prepares := range pset {
		if idx.n > rbft.exec.lastExec {
			if clean {
				rbft.logger.Debugf("Replica %d clean pSet with seqNo %d > lastExec %d", rbft.peerPool.ID, idx.n, rbft.exec.lastExec)
				rbft.persistDelPSet(idx.v, idx.n, idx.d)
				continue
			}
			rbft.logger.Debugf("Replica %d restore pSet with seqNo %d > lastExec %d", rbft.peerPool.ID, idx.n, rbft.exec.lastExec)
		}
		cert := rbft.storeMgr.getCert(idx.v, idx.n, idx.d)
		for _, p := range prepares.Set {
			cert.prepare[*p] = true
			if p.ReplicaId == rbft.peerPool.ID && idx.n <= rbft.exec.lastExec {
				cert.sentPrepare = true
			}
		}
	}

	cset, _ := rbft.restoreCSet()
	for idx, commits := range cset {
		if idx.n > rbft.exec.lastExec {
			if clean {
				rbft.logger.Debugf("Replica %d clean cSet with seqNo %d > lastExec %d", rbft.peerPool.ID, idx.n, rbft.exec.lastExec)
				rbft.persistDelCSet(idx.v, idx.n, idx.d)
				continue
			}
			rbft.logger.Debugf("Replica %d restore cSet with seqNo %d > lastExec %d", rbft.peerPool.ID, idx.n, rbft.exec.lastExec)
		}
		cert := rbft.storeMgr.getCert(idx.v, idx.n, idx.d)
		for _, c := range commits.Set {
			cert.commit[*c] = true
			if c.ReplicaId == rbft.peerPool.ID && idx.n <= rbft.exec.lastExec {
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
func (rbft *rbftImpl) persistBatch(digest string) {
	batch := rbft.storeMgr.batchStore[digest]
	batchPacked, err := proto.Marshal(batch)
	if err != nil {
		rbft.logger.Warningf("Replica %d could not persist request batch %s: %s", rbft.peerPool.ID, digest, err)
		return
	}
	start := time.Now()
	err = rbft.storage.StoreState("batch."+digest, batchPacked)
	if err != nil {
		rbft.logger.Errorf("Persist batch failed with err: %s ", err)
	}
	duration := time.Now().Sub(start).Seconds()
	rbft.metrics.batchPersistDuration.Observe(duration)
}

// persistDelBatch removes one marshaled tx batch with the given digest from database
func (rbft *rbftImpl) persistDelBatch(digest string) {
	_ = rbft.storage.DelState("batch." + digest)
}

// persistDelAllBatches removes all marshaled tx batches from database
func (rbft *rbftImpl) persistDelAllBatches() {
	_ = rbft.storage.Destroy("batch")
}

// persistCheckpoint persists checkpoint to database, which, key contains the seqNo of checkpoint, value is the
// checkpoint ID
func (rbft *rbftImpl) persistCheckpoint(seqNo uint64, id []byte) {
	key := fmt.Sprintf("chkpt.%d", seqNo)
	err := rbft.storage.StoreState(key, id)
	if err != nil {
		rbft.logger.Errorf("Persist chkpt failed with err: %s ", err)
	}
}

// persistDelCheckpoint deletes checkpoint with the given seqNo from database
func (rbft *rbftImpl) persistDelCheckpoint(seqNo uint64) {
	key := fmt.Sprintf("chkpt.%d", seqNo)
	_ = rbft.storage.DelState(key)
}

func (rbft *rbftImpl) persistH(seqNo uint64) {
	err := rbft.storage.StoreState("rbft.h", []byte(strconv.FormatUint(seqNo, 10)))
	if err != nil {
		rbft.logger.Errorf("Persist h failed with err: %s ", err)
	}
}

// persistView persists current view to database
func (rbft *rbftImpl) persistView(view uint64) {
	key := "view"
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, view)
	err := rbft.storage.StoreState(key, b)
	if err != nil {
		rbft.logger.Errorf("Persist view failed with err: %s ", err)
	}
}

// persistN persists current N to database
func (rbft *rbftImpl) persistN(n int) {
	key := "nodes"
	res := make([]byte, 8)
	binary.LittleEndian.PutUint64(res, uint64(n))
	err := rbft.storage.StoreState(key, res)
	if err != nil {
		rbft.logger.Errorf("Persist N failed with err: %s ", err)
	}
}

// restoreN restore current N from database
func (rbft *rbftImpl) restoreN() {
	n, err := rbft.storage.ReadState("nodes")
	if err == nil {
		nodes := binary.LittleEndian.Uint64(n)
		rbft.N = int(nodes)
		rbft.f = (rbft.N - 1) / 3
	}
	rbft.logger.Noticef("========= restore N=%d, f=%d =======", rbft.N, rbft.f)
}

// restoreView restores current view from database and then re-construct certStore
func (rbft *rbftImpl) restoreView() bool {
	setView, err := rbft.storage.ReadState("setView")
	// delete this key immediately no matter this key exists or not to totally avoid
	// setView again in next restart.
	_ = rbft.storage.DelState("setView")
	if err == nil && string(setView) != "" {
		var nv int
		nv, err = strconv.Atoi(string(setView))
		if err != nil {
			rbft.logger.Warningf("Replica %d could not restore setView %s to a integer", rbft.peerPool.ID, string(setView))
		} else {
			rbft.setView(uint64(nv))
			rbft.logger.Noticef("========= restore set view %d =======", rbft.view)
			return true
		}
	}

	v, err := rbft.storage.ReadState("view")
	if err == nil {
		view := binary.LittleEndian.Uint64(v)
		rbft.setView(view)
		rbft.logger.Noticef("========= restore view %d =======", rbft.view)
	} else {
		rbft.logger.Warningf("Replica %d could not restore view: %s, set to 0", rbft.peerPool.ID, err)
		rbft.setView(uint64(0))
	}
	return false
}

// restoreBatchStore restores tx batches from database
func (rbft *rbftImpl) restoreBatchStore() {

	payload, err := rbft.storage.ReadStateSet("batch.")
	if err == nil {
		for key, set := range payload {
			var digest string
			if _, err = fmt.Sscanf(key, "batch.%s", &digest); err != nil {
				rbft.logger.Warningf("Replica %d could not restore pset key %s", rbft.peerPool.ID, key)
			} else {
				batch := &pb.RequestBatch{}
				err = proto.Unmarshal(set, batch)
				if err == nil {
					rbft.logger.Debugf("Replica %d restore batch %s", rbft.peerPool.ID, digest)
					rbft.storeMgr.batchStore[digest] = batch
					rbft.metrics.batchesGauge.Add(float64(1))
				} else {
					rbft.logger.Warningf("Replica %d could not unmarshal batch key %s for error: %v", rbft.peerPool.ID, key, err)
				}
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d could not restore batch: %v", rbft.peerPool.ID, err)
	}
}

// It is application's responsibility to ensure data compatibility, so RBFT core need only trust and restore
// consensus data from consensus DB.
// restoreState restores lastExec, certStore, view, transaction batches, checkpoints, h and other add/del node related
// params from database
func (rbft *rbftImpl) restoreState() error {

	// TODO(DH): move router restore to external.
	rbft.batchMgr.setSeqNo(rbft.exec.lastExec)
	setView := rbft.restoreView()
	rbft.restoreN()

	rbft.restoreCert()

	// TODO(DH): do we need to save setView?
	if setView {
		rbft.parseCertStore()
	}
	rbft.restoreBatchStore()

	chkpts, err := rbft.storage.ReadStateSet("chkpt.")
	if err == nil {
		for key, id := range chkpts {
			var seqNo uint64
			if _, err = fmt.Sscanf(key, "chkpt.%d", &seqNo); err != nil {
				rbft.logger.Warningf("Replica %d could not restore checkpoint key %s", rbft.peerPool.ID, key)
			} else {
				digest := string(id)
				rbft.logger.Debugf("Replica %d found checkpoint %s for seqNo %d", rbft.peerPool.ID, digest, seqNo)
				rbft.storeMgr.saveCheckpoint(seqNo, digest)
			}
		}
	} else {
		rbft.logger.Warningf("Replica %d could not restore checkpoints: %s", rbft.peerPool.ID, err)
	}

	hstr, err := rbft.storage.ReadState("rbft.h")
	if err != nil {
		rbft.logger.Warningf("Replica %d could not restore h: %s", rbft.peerPool.ID, err)
	} else {
		h, err := strconv.ParseUint(string(hstr), 10, 64)
		if err != nil {
			rbft.logger.Warningf("transfer rbft.h from string to uint64 failed with err: %s", err)
			return err
		}
		rbft.moveWatermarks(h)
	}

	rbft.logger.Infof("Replica %d restored state: view: %d, seqNo: %d, reqBatches: %d, chkpts: %d",
		rbft.peerPool.ID, rbft.view, rbft.exec.lastExec, len(rbft.storeMgr.batchStore), len(rbft.storeMgr.chkpts))

	return nil
}

// parseCertStore parses certStore and remove the cert with the same seqNo but
// a lower view.
func (rbft *rbftImpl) parseCertStore() {
	// parse certStore
	rbft.logger.Debugf("Replica %d parse certStore to view %d", rbft.peerPool.ID, rbft.view)
	newCertStore := make(map[msgID]*msgCert)
	for idx, cert := range rbft.storeMgr.certStore {
		maxIdx := idx
		maxCert := cert
		for tmpIdx, tmpCert := range rbft.storeMgr.certStore {
			if maxIdx.n == tmpIdx.n {
				if maxIdx.v <= tmpIdx.v {
					maxIdx = tmpIdx
					maxCert = tmpCert
					rbft.persistDelQPCSet(tmpIdx.v, tmpIdx.n, tmpIdx.d)
				}
			}
		}
		if maxCert.prePrepare != nil {
			maxCert.prePrepare.View = rbft.view
			primaryID := rbft.primaryID(rbft.view)
			maxCert.prePrepare.ReplicaId = primaryID
			rbft.persistQSet(maxCert.prePrepare)
		} else {
			rbft.logger.Debugf("Replica %d finds nil prePrepare with view=%d/seqNo=%d/digest=%s", rbft.peerPool.ID, maxIdx.v, maxIdx.n, maxIdx.d)
		}
		preps := make(map[pb.Prepare]bool)
		for prep := range maxCert.prepare {
			prep.View = rbft.view
			preps[prep] = true
		}
		maxCert.prepare = preps
		rbft.persistPSet(maxIdx.v, maxIdx.n, maxIdx.d)

		cmts := make(map[pb.Commit]bool)
		for cmt := range maxCert.commit {
			cmt.View = rbft.view
			cmts[cmt] = true
		}
		maxCert.commit = cmts
		rbft.persistCSet(maxIdx.v, maxIdx.n, maxIdx.d)

		maxIdx.v = rbft.view
		if maxIdx.n > rbft.exec.lastExec {
			maxCert.sentPrepare = false
			maxCert.sentCommit = false
			maxCert.sentExecute = false
		}
		newCertStore[maxIdx] = maxCert
	}
	rbft.storeMgr.certStore = newCertStore

	// parse pqlist
	rbft.logger.Debugf("Replica %d parse pqlist to view %d", rbft.peerPool.ID, rbft.view)
	for _, prepare := range rbft.vcMgr.plist {
		prepare.View = rbft.view
	}

	for _, prePrepare := range rbft.vcMgr.qlist {
		prePrepare.View = rbft.view
	}
}

// parseQPCKey helps parse view, seqNo, digest from given key with prefix.
func (rbft *rbftImpl) parseQPCKey(key, prefix string) (uint64, uint64, string, error) {
	var (
		v, n int
		d    string
		err  error
	)

	splitKeys := strings.Split(key, ".")
	if len(splitKeys) != 4 {
		rbft.logger.Warningf("Replica %d could not restore key %s with prefix %s", rbft.peerPool.ID, key, prefix)
		return 0, 0, "", errors.New("incorrect format")
	}

	if splitKeys[0] != prefix {
		rbft.logger.Errorf("Replica %d finds error key prefix when restore %s using %s", rbft.peerPool.ID, prefix, key)
		return 0, 0, "", errors.New("incorrect prefix")
	}

	v, err = strconv.Atoi(splitKeys[1])
	if err != nil {
		rbft.logger.Errorf("Replica %d could not parse key %s to int", rbft.peerPool.ID, splitKeys[1])
		return 0, 0, "", errors.New("parse failed")
	}

	n, err = strconv.Atoi(splitKeys[2])
	if err != nil {
		rbft.logger.Errorf("Replica %d could not parse key %s to int", rbft.peerPool.ID, splitKeys[2])
		return 0, 0, "", errors.New("parse failed")
	}

	d = splitKeys[3]
	return uint64(v), uint64(n), d, nil
}
