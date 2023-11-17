package rbft

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	types2 "github.com/axiomesh/axiom-kit/types"

	"github.com/axiomesh/axiom-bft/common"
	"github.com/axiomesh/axiom-bft/common/consensus"
	"github.com/axiomesh/axiom-bft/common/metrics/disabled"
	"github.com/axiomesh/axiom-bft/types"
)

func newTestTimerMgr[T any, Constraint types2.TXConstraint[T]](ctrl *gomock.Controller) *timerManager {
	log := common.NewSimpleLogger()
	conf := Config{
		LastServiceState: &types.ServiceState{
			MetaState: &types.MetaState{},
			Epoch:     1,
		},
		SelfAccountAddress: "node1",
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
				CheckpointPeriod:              2,
				HighWatermarkCheckpointPeriod: 2,
				MaxValidatorNum:               10,
				BlockMaxTxNum:                 500,
				NotActiveWeight:               1,
				AbnormalNodeExcludeView:       10,
			},
		},
		Logger:      log,
		MetricsProv: &disabled.Provider{},
		DelFlag:     make(chan bool),
	}
	eventC := make(chan consensusEvent)

	return newTimerMgr(eventC, conf)
}

func TestTimerMgr_store(t *testing.T) {
	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	value1 := time.AfterFunc(5*time.Second, func() {})
	tt.store("key1", value1)
	v, _ := tt.isActive.Load("key1")
	assert.Equal(t, value1, v)
}

func TestTimerMgr_delete(t *testing.T) {
	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}

	value1 := time.AfterFunc(5*time.Second, func() {})

	tt.store("key1", value1)
	tt.delete("key1")
	v, _ := tt.isActive.Load("key1")
	assert.Equal(t, nil, v)
}

func TestTimerMgr_count(t *testing.T) {
	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	value1 := time.AfterFunc(5*time.Second, func() {})

	tt.store("key1", value1)
	assert.Equal(t, 1, tt.count())
}

func TestTimerMgr_stopTimer(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	timeMgr := newTestTimerMgr[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}

	value1 := time.AfterFunc(5*time.Second, func() {})

	tt.store("key1", value1)
	timeMgr.tTimers["test1"] = &tt

	timeMgr.stopTimer("null timerMgr")
	assert.Equal(t, 1, timeMgr.tTimers["test1"].count())

	timeMgr.stopTimer("test1")
	assert.Equal(t, 0, timeMgr.tTimers["test1"].count())
}

func TestTimerMgr_stopOneTimer(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	timeMgr := newTestTimerMgr[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	value1 := time.AfterFunc(5*time.Second, func() {})
	value2 := time.AfterFunc(2*time.Second, func() {})
	tt.store("key1", value1)
	tt.store("key2", value2)
	timeMgr.tTimers["test1"] = &tt

	timeMgr.stopOneTimer("null timerMgr", "key1")
	assert.Equal(t, true, timeMgr.tTimers["test1"].has("key1"))

	timeMgr.stopOneTimer("test1", "key1")
	assert.Equal(t, false, timeMgr.tTimers["test1"].has("key1"))
}

func TestTimerMgr_getTimeoutValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	timeMgr := newTestTimerMgr[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}

	value1 := time.AfterFunc(5*time.Second, func() {})
	value2 := time.AfterFunc(2*time.Second, func() {})
	tt.store("key1", value1)
	tt.store("key2", value2)
	timeMgr.tTimers["test1"] = &tt

	assert.Equal(t, time.Duration(0), timeMgr.getTimeoutValue("null timerMgr"))
	assert.Equal(t, time.Duration(5), timeMgr.getTimeoutValue("test1"))
}

func TestTimerMgr_setTimeoutValue(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	timeMgr := newTestTimerMgr[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}

	value1 := time.AfterFunc(5*time.Second, func() {})
	value2 := time.AfterFunc(2*time.Second, func() {})
	tt.store("key1", value1)
	tt.store("key2", value2)
	timeMgr.tTimers["test1"] = &tt

	timeMgr.setTimeoutValue("null", time.Duration(10))
	assert.Equal(t, time.Duration(5), timeMgr.getTimeoutValue("test1"))
	timeMgr.setTimeoutValue("test1", time.Duration(10))
	assert.Equal(t, time.Duration(10), timeMgr.getTimeoutValue("test1"))
}

func TestTimerMgr_makeNullRequestTimeoutLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	timeMgr := newTestTimerMgr[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	timeMgr.tTimers[nullRequestTimer] = &tt
	timeMgr.tTimers[requestTimer] = &tt

	timeMgr.setTimeoutValue(nullRequestTimer, time.Duration(5))
	timeMgr.setTimeoutValue(requestTimer, time.Duration(10))
	timeMgr.makeNullRequestTimeoutLegal()
	assert.Equal(t, time.Duration(15), timeMgr.getTimeoutValue(nullRequestTimer))
}

func TestTimerMgr_makeCleanVcTimeoutLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	timeMgr := newTestTimerMgr[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	timeMgr.tTimers[cleanViewChangeTimer] = &tt
	timeMgr.tTimers[newViewTimer] = &tt

	timeMgr.setTimeoutValue(cleanViewChangeTimer, time.Duration(5))
	timeMgr.setTimeoutValue(newViewTimer, time.Duration(1))
	timeMgr.makeCleanVcTimeoutLegal()
	assert.Equal(t, time.Duration(6), timeMgr.getTimeoutValue(cleanViewChangeTimer))
}

func TestTimerMgr_makeSyncStateTimeoutLegal(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	timeMgr := newTestTimerMgr[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	tt := titleTimer{
		timerName: "test timer",
		timeout:   5,
		isActive:  sync.Map{},
	}
	timeMgr.tTimers[syncStateRspTimer] = &tt
	timeMgr.tTimers[syncStateRestartTimer] = &tt

	timeMgr.setTimeoutValue(syncStateRspTimer, time.Duration(5))
	timeMgr.setTimeoutValue(syncStateRestartTimer, time.Duration(1))
	timeMgr.makeSyncStateTimeoutLegal()
	assert.Equal(t, time.Duration(10), timeMgr.getTimeoutValue(syncStateRspTimer))
}

func TestTimerMgr_newTimer(t *testing.T) {
	ctrl := gomock.NewController(t)
	// defer ctrl.Finish()

	timeMgr := newTestTimerMgr[consensus.FltTransaction, *consensus.FltTransaction](ctrl)

	timeMgr.newTimer(requestTimer, 0)
	timeMgr.newTimer(batchTimer, 0)
	timeMgr.newTimer(vcResendTimer, 0)
	timeMgr.newTimer(newViewTimer, 0)
	timeMgr.newTimer(nullRequestTimer, 0)
	timeMgr.newTimer(syncStateRspTimer, 0)
	timeMgr.newTimer(syncStateRestartTimer, 0)
	timeMgr.newTimer(cleanViewChangeTimer, 0)
	timeMgr.newTimer(checkPoolTimer, 0)
	timeMgr.newTimer(fetchCheckpointTimer, 0)
	assert.Equal(t, DefaultRequestTimeout, timeMgr.tTimers[requestTimer].timeout)
	assert.Equal(t, DefaultBatchTimeout, timeMgr.tTimers[batchTimer].timeout)
	assert.Equal(t, DefaultVcResendTimeout, timeMgr.tTimers[vcResendTimer].timeout)
	assert.Equal(t, DefaultNewViewTimeout, timeMgr.tTimers[newViewTimer].timeout)
	assert.Equal(t, DefaultNullRequestTimeout, timeMgr.tTimers[nullRequestTimer].timeout)
	assert.Equal(t, DefaultSyncStateRspTimeout, timeMgr.tTimers[syncStateRspTimer].timeout)
	assert.Equal(t, DefaultSyncStateRestartTimeout, timeMgr.tTimers[syncStateRestartTimer].timeout)
	assert.Equal(t, DefaultCleanViewChangeTimeout, timeMgr.tTimers[cleanViewChangeTimer].timeout)
	assert.Equal(t, DefaultCheckPoolTimeout, timeMgr.tTimers[checkPoolTimer].timeout)
	assert.Equal(t, DefaultFetchCheckpointTimeout, timeMgr.tTimers[fetchCheckpointTimer].timeout)
}
