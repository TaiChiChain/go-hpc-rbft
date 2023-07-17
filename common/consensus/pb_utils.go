package consensus

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"golang.org/x/crypto/sha3"
)

// Validators is a list of NodeInfo
type Validators = []*NodeInfo

// ValidateSetEquals compares two sets of validator and returns whether they are equal
func ValidateSetEquals(a, b Validators) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i].Hostname != b[i].Hostname {
			return false
		}
		if !bytes.Equal(a[i].PubKey, b[i].PubKey) {
			return false
		}
	}
	return true
}

func consensusStateEquals(a, b *Checkpoint_ConsensusState) bool {
	return a.GetRound() == b.GetRound() && a.GetId() == b.GetId()
}

func executeStateEquals(a, b *Checkpoint_ExecuteState) bool {
	return a.GetHeight() == b.GetHeight() && a.GetDigest() == b.GetDigest()
}

func epochStateEquals(a, b *Checkpoint_NextEpochState) bool {
	return ValidateSetEquals(a.GetValidatorSet(), b.GetValidatorSet())
}

// ======================== NodeInfo ========================
// String returns a formatted string for NodeInfo.
func (m *NodeInfo) String() string {
	if m == nil {
		return "NIL"
	}
	return fmt.Sprintf("{host:%s, key:%s}", m.GetHostname(), hex.EncodeToString(m.GetPubKey()))
}

// ======================= Checkpoint =======================

// Hash calculates the crypto hash of Checkpoint.
func (m *Checkpoint) Hash() []byte {
	if m == nil {
		return nil
	}
	res, jErr := proto.Marshal(m)
	if jErr != nil {
		panic(jErr)
	}
	hasher := sha3.NewLegacyKeccak256()
	//nolint
	hasher.Write(res)
	h := hasher.Sum(nil)
	return h
}

// Round returns round of checkpoint.
func (m *Checkpoint) Round() uint64 {
	return m.GetConsensusState().GetRound()
}

// ID returns id of checkpoint.
func (m *Checkpoint) ID() string {
	return m.GetConsensusState().GetId()
}

// Height returns height of checkpoint.
func (m *Checkpoint) Height() uint64 {
	return m.GetExecuteState().GetHeight()
}

// Digest returns digest of checkpoint.
func (m *Checkpoint) Digest() string {
	return m.GetExecuteState().GetDigest()
}

// SetDigest set digest to checkpoint.
func (m *Checkpoint) SetDigest(digest string) {
	if m == nil {
		return
	}
	m.ExecuteState.Digest = digest
}

// ValidatorSet returns vset of checkpoint.
func (m *Checkpoint) ValidatorSet() Validators {
	return m.GetNextEpochState().GetValidatorSet()
}

// ConsensusVersion returns consensus version of checkpoint.
func (m *Checkpoint) ConsensusVersion() string {
	return m.GetNextEpochState().GetConsensusVersion()
}

// SetValidatorSet set vset to checkpoint.
func (m *Checkpoint) SetValidatorSet(vset Validators) {
	if m == nil {
		return
	}
	if m.NextEpochState == nil {
		m.NextEpochState = &Checkpoint_NextEpochState{}
	}
	m.NextEpochState.ValidatorSet = vset
}

// Version returns version of checkpoint.
func (m *Checkpoint) Version() string {
	return m.GetNextEpochState().GetConsensusVersion()
}

// SetVersion set version to checkpoint.
func (m *Checkpoint) SetVersion(version string) {
	if m == nil {
		return
	}
	if m.NextEpochState == nil {
		m.NextEpochState = &Checkpoint_NextEpochState{}
	}
	m.NextEpochState.ConsensusVersion = version
}

// Reconfiguration returns whether this is a config checkpoint
func (m *Checkpoint) Reconfiguration() bool {
	return m.GetNextEpochState() != nil
}

// EndsEpoch returns whether this checkpoint ends the epoch
func (m *Checkpoint) EndsEpoch() bool {
	// TODO(YC): would each reconfiguration end epoch?
	return m.GetNextEpochState() != nil
}

// NextEpoch returns epoch change to checkpoint.
func (m *Checkpoint) NextEpoch() uint64 {
	epoch := m.GetEpoch()
	if m.EndsEpoch() {
		epoch++
	}
	return epoch
}

// String returns a formatted string for Checkpoint.
func (m *Checkpoint) String() string {
	if m == nil {
		return "NIL"
	}
	return fmt.Sprintf("epoch: %d, round: %d, height: %d, hash: %s, commit block id: %s, "+
		"validator set %v, consensus version %s", m.GetEpoch(), m.Round(), m.Height(), m.Digest(),
		m.ID(), m.ValidatorSet(), m.ConsensusVersion())
}

// Equals compares two checkpoint instance and returns whether they are equal
func (m *Checkpoint) Equals(n *Checkpoint) bool {
	return m.GetEpoch() == n.GetEpoch() &&
		consensusStateEquals(m.GetConsensusState(), n.GetConsensusState()) &&
		executeStateEquals(m.GetExecuteState(), n.GetExecuteState()) &&
		epochStateEquals(m.GetNextEpochState(), n.GetNextEpochState())
}

// ======================= SignedCheckpoint =======================

// Hash calculates the crypto hash of SignedCheckpoint.
func (m *SignedCheckpoint) Hash() []byte {
	return m.GetCheckpoint().Hash()
}

// Round returns round of signed checkpoint.
func (m *SignedCheckpoint) Round() uint64 {
	return m.GetCheckpoint().Round()
}

// Epoch returns epoch of signed checkpoint.
func (m *SignedCheckpoint) Epoch() uint64 {
	return m.GetCheckpoint().GetEpoch()
}

// Height returns height of signed checkpoint.
func (m *SignedCheckpoint) Height() uint64 {
	return m.GetCheckpoint().Height()
}

// Digest returns digest of signed checkpoint.
func (m *SignedCheckpoint) Digest() string {
	return m.GetCheckpoint().Digest()
}

// ValidatorSet returns validators of signed checkpoint.
func (m *SignedCheckpoint) ValidatorSet() Validators {
	return m.GetCheckpoint().ValidatorSet()
}

// String returns a formatted string for SignedCheckpoint.
func (m *SignedCheckpoint) String() string {
	if m == nil {
		return "NIL"
	}
	return fmt.Sprintf("Checkpoint %s signed by %s", m.GetCheckpoint(), m.GetAuthor())
}

// ======================= QuorumCheckpoint =======================

// Hash calculates the crypto hash of QuorumCheckpoint.
func (m *QuorumCheckpoint) Hash() []byte {
	return m.GetCheckpoint().Hash()
}

// ID returns id of quorum checkpoint.
func (m *QuorumCheckpoint) ID() string {
	return m.GetCheckpoint().ID()
}

// Digest returns digest of quorum checkpoint.
func (m *QuorumCheckpoint) Digest() string {
	return m.GetCheckpoint().Digest()
}

// ValidatorSet returns validators of quorum checkpoint.
func (m *QuorumCheckpoint) ValidatorSet() Validators {
	return m.GetCheckpoint().ValidatorSet()
}

// Version returns version of quorum checkpoint.
func (m *QuorumCheckpoint) Version() string {
	return m.GetCheckpoint().Version()
}

// Epoch returns epoch of quorum checkpoint.
func (m *QuorumCheckpoint) Epoch() uint64 {
	return m.GetCheckpoint().GetEpoch()
}

// PrevEpoch returns previous epoch change of quorum checkpoint.
func (m *QuorumCheckpoint) PrevEpoch() uint64 {
	epoch := m.Epoch()
	if epoch > 0 && m.EndsEpoch() {
		epoch--
	}
	return epoch
}

// NextEpoch returns epoch change to of quorum checkpoint.
func (m *QuorumCheckpoint) NextEpoch() uint64 {
	return m.GetCheckpoint().NextEpoch()
}

// Round returns round of quorum checkpoint.
func (m *QuorumCheckpoint) Round() uint64 {
	return m.GetCheckpoint().Round()
}

// Height returns height of quorum checkpoint.
func (m *QuorumCheckpoint) Height() uint64 {
	return m.GetCheckpoint().Height()
}

// Reconfiguration returns whether this is a config checkpoint
func (m *QuorumCheckpoint) Reconfiguration() bool {
	return m.GetCheckpoint().Reconfiguration()
}

// EndsEpoch returns whether this checkpoint ends the epoch
func (m *QuorumCheckpoint) EndsEpoch() bool {
	return m.GetCheckpoint().EndsEpoch()
}

// AddSignature adds a certified signature to QuorumCheckpoint.
func (m *QuorumCheckpoint) AddSignature(validator string, signature []byte) {
	if m == nil {
		return
	}
	m.Signatures[validator] = signature
}

// String returns a formatted string for LedgerInfoWithSignatures.
func (m *QuorumCheckpoint) String() string {
	if m == nil {
		return "NIL"
	}
	authors := make([]string, len(m.GetSignatures()))
	i := 0
	for author := range m.GetSignatures() {
		authors[i] = author
		i++
	}
	return fmt.Sprintf("CheckpointInfo: %s signed by: %+v", m.GetCheckpoint(), authors)
}

// ======================= EpochChangeProof =======================

// NewEpochChangeProof create a new proof with given checkpoints
func NewEpochChangeProof(checkpoints ...*QuorumCheckpoint) *EpochChangeProof {
	// checkpoints should end epoch
	return &EpochChangeProof{Checkpoints: checkpoints}
}

// StartEpoch returns start epoch of the proof
func (m *EpochChangeProof) StartEpoch() uint64 {
	return m.First().Epoch()
}

// NextEpoch returns the next epoch to change to of the proof
func (m *EpochChangeProof) NextEpoch() uint64 {
	return m.Last().NextEpoch()
}

// First returns the first checkpoint of the proof
func (m *EpochChangeProof) First() *QuorumCheckpoint {
	if m.IsEmpty() {
		return nil
	}
	return m.Checkpoints[0]
}

// Last returns the last checkpoint of the proof
func (m *EpochChangeProof) Last() *QuorumCheckpoint {
	if m.IsEmpty() {
		return nil
	}
	return m.Checkpoints[len(m.Checkpoints)-1]
}

// IsEmpty returns whether the proof is empty
func (m *EpochChangeProof) IsEmpty() bool {
	if m == nil {
		return true
	}
	return len(m.GetCheckpoints()) == 0
}

// String returns a formatted string for EpochChangeProof.
func (m *EpochChangeProof) String() string {
	if m == nil {
		return "NIL"
	}
	return fmt.Sprintf("EpochChangeProof: checkpoints %s, more %d, author %s", m.GetCheckpoints(), m.GetMore(), m.GetAuthor())
}
