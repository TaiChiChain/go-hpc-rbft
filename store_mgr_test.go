package rbft

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/common/metrics/disabled"
)

func newStorageTestNode[T any, Constraint consensus.TXConstraint[T]](ctrl *gomock.Controller) (*storeManager[T, Constraint], Config) {
	log := common.NewSimpleLogger()

	conf := Config{
		SelfAccountAddress: "node2",
		GenesisEpochInfo: &EpochInfo{
			Version:                   1,
			Epoch:                     1,
			EpochPeriod:               1000,
			CandidateSet:              []NodeInfo{},
			ValidatorSet:              peerSet,
			StartBlock:                1,
			P2PBootstrapNodeAddresses: []string{},
			ConsensusParams: ConsensusParams{
				ValidatorElectionType:         ValidatorElectionTypeWRF,
				ProposerElectionType:          ProposerElectionTypeAbnormalRotation,
				CheckpointPeriod:              10,
				HighWatermarkCheckpointPeriod: 4,
				MaxValidatorNum:               10,
				BlockMaxTxNum:                 500,
				NotActiveWeight:               1,
				ExcludeView:                   10,
			},
		},
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

		Logger:      log,
		MetricsProv: &disabled.Provider{},
		DelFlag:     make(chan bool),
	}

	return newStoreMgr[T, Constraint](conf), conf
}

func TestStoreMgr_getCert(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	s, _ := newStorageTestNode[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	var retCert *msgCert
	// get default cert
	certDefault := &msgCert{
		prePrepare:  nil,
		sentPrepare: false,
		prepare:     make(map[string]*consensus.Prepare),
		sentCommit:  false,
		commit:      make(map[string]*consensus.Commit),
		sentExecute: false,
	}

	retCert = s.getCert(1, 10, "default")
	assert.Equal(t, certDefault, retCert)

	msgIDTmp := msgID{
		v: 1,
		n: 20,
		d: "tmp",
	}
	certTmp := &msgCert{
		prePrepare:  nil,
		sentPrepare: true,
		prepare:     make(map[string]*consensus.Prepare),
		sentCommit:  true,
		commit:      make(map[string]*consensus.Commit),
		sentExecute: true,
	}
	s.certStore[msgIDTmp] = certTmp
	retCert = s.getCert(1, 20, "tmp")
	assert.Equal(t, certTmp, retCert)
}

func TestStoreMgr_existedDigest(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	s, _ := newStorageTestNode[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	msgIDTmp := msgID{
		v: 1,
		n: 20,
		d: "tmp",
	}
	prePrepareTmp := &consensus.PrePrepare{
		ReplicaId:      2,
		View:           0,
		SequenceNumber: 20,
		BatchDigest:    "tmp",
		HashBatch:      nil,
	}
	certTmp := &msgCert{
		prePrepare:  prePrepareTmp,
		sentPrepare: true,
		prepare:     make(map[string]*consensus.Prepare),
		sentCommit:  true,
		commit:      make(map[string]*consensus.Commit),
		sentExecute: true,
	}
	s.certStore[msgIDTmp] = certTmp

	assert.Equal(t, false, s.existedDigest(0, 20, "tmp"))
	assert.Equal(t, true, s.existedDigest(0, 10, "tmp"))
}
