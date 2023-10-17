package consensus

import (
	"fmt"

	"golang.org/x/crypto/sha3"
)

type Message interface {
	MarshalVTStrict() (dAtA []byte, err error)
	MarshalVT() (dAtA []byte, err error)
	UnmarshalVT(dAtA []byte) error
}

func executeStateEquals(a, b *Checkpoint_ExecuteState) bool {
	return a.GetHeight() == b.GetHeight() && a.GetDigest() == b.GetDigest()
}

// ======================= Checkpoint =======================

// Hash calculates the crypto hash of Checkpoint.
func (m *Checkpoint) Hash() []byte {
	if m == nil {
		return nil
	}
	c := &Checkpoint{
		Epoch:           m.Epoch,
		ExecuteState:    m.ExecuteState,
		NeedUpdateEpoch: m.NeedUpdateEpoch,
	}
	res, jErr := c.MarshalVTStrict()
	if jErr != nil {
		panic(jErr)
	}
	hasher := sha3.NewLegacyKeccak256()
	//nolint
	hasher.Write(res)
	h := hasher.Sum(nil)
	return h
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

// NextEpoch returns epoch change to checkpoint.
func (m *Checkpoint) NextEpoch() uint64 {
	epoch := m.GetEpoch()
	if m.NeedUpdateEpoch {
		epoch++
	}
	return epoch
}

// Pretty returns a formatted string for Checkpoint.
func (m *Checkpoint) Pretty() string {
	if m == nil {
		return "NIL"
	}
	return fmt.Sprintf("{epoch: %d,  height: %d, hash: %s}", m.GetEpoch(), m.Height(), m.Digest())
}

// Equals compares two checkpoint instance and returns whether they are equal
func (m *Checkpoint) Equals(n *Checkpoint) bool {
	return m.Epoch == n.Epoch &&
		executeStateEquals(m.GetExecuteState(), n.GetExecuteState()) && m.NeedUpdateEpoch == n.NeedUpdateEpoch
}

// ======================= SignedCheckpoint =======================

// Hash calculates the crypto hash of SignedCheckpoint.
func (m *SignedCheckpoint) Hash() []byte {
	return m.GetCheckpoint().Hash()
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

// Pretty returns a formatted string for SignedCheckpoint.
func (m *SignedCheckpoint) Pretty() string {
	if m == nil {
		return "NIL"
	}
	return fmt.Sprintf("Checkpoint %s signed by %d", m.GetCheckpoint().Pretty(), m.GetAuthor())
}

// ======================= QuorumCheckpoint =======================

// Hash calculates the crypto hash of QuorumCheckpoint.
func (m *QuorumCheckpoint) Hash() []byte {
	return m.GetCheckpoint().Hash()
}

// Digest returns digest of quorum checkpoint.
func (m *QuorumCheckpoint) Digest() string {
	return m.GetCheckpoint().Digest()
}

// Epoch returns epoch of quorum checkpoint.
func (m *QuorumCheckpoint) Epoch() uint64 {
	return m.GetCheckpoint().GetEpoch()
}

func (m *QuorumCheckpoint) NeedUpdateEpoch() bool {
	return m.GetCheckpoint().NeedUpdateEpoch
}

// NextEpoch returns epoch change to of quorum checkpoint.
func (m *QuorumCheckpoint) NextEpoch() uint64 {
	return m.GetCheckpoint().NextEpoch()
}

// Height returns height of quorum checkpoint.
func (m *QuorumCheckpoint) Height() uint64 {
	return m.GetCheckpoint().Height()
}

// AddSignature adds a certified signature to QuorumCheckpoint.
func (m *QuorumCheckpoint) AddSignature(validator uint64, signature []byte) {
	if m == nil {
		return
	}
	m.Signatures[validator] = signature
}

// Pretty returns a formatted string for LedgerInfoWithSignatures.
func (m *QuorumCheckpoint) Pretty() string {
	if m == nil {
		return "NIL"
	}
	authors := make([]uint64, len(m.GetSignatures()))
	i := 0
	for author := range m.GetSignatures() {
		authors[i] = author
		i++
	}
	return fmt.Sprintf("CheckpointInfo: %s signed by: %+v", m.GetCheckpoint().Pretty(), authors)
}

// ======================= EpochChangeProof =======================

// StartEpoch returns start epoch of the proof
func (m *EpochChangeProof) StartEpoch() uint64 {
	return m.First().Epoch()
}

// NextEpoch returns the next epoch to change to of the proof
func (m *EpochChangeProof) NextEpoch() uint64 {
	return m.Last().Checkpoint.NextEpoch()
}

// First returns the first checkpoint of the proof
func (m *EpochChangeProof) First() *QuorumCheckpoint {
	if m.IsEmpty() {
		return nil
	}
	return m.EpochChanges[0].Checkpoint
}

// Last returns the last checkpoint of the proof
func (m *EpochChangeProof) Last() *EpochChange {
	if m.IsEmpty() {
		return nil
	}
	return m.EpochChanges[len(m.EpochChanges)-1]
}

// IsEmpty returns whether the proof is empty
func (m *EpochChangeProof) IsEmpty() bool {
	if m == nil {
		return true
	}
	return len(m.GetEpochChanges()) == 0
}

// Pretty returns a formatted string for EpochChangeProof.
func (m *EpochChangeProof) Pretty() string {
	if m == nil {
		return "NIL"
	}
	return fmt.Sprintf("EpochChangeProof: epochChange:%v, more %d, author %d", m.GetEpochChanges(), m.GetMore(), m.GetAuthor())
}

func (x *Prepare) ID() string {
	return fmt.Sprintf("replica=%d/view=%d/seq=%d/digest=%s", x.ReplicaId, x.View, x.SequenceNumber, x.BatchDigest)
}

func (x *Commit) ID() string {
	return fmt.Sprintf("replica=%d/view=%d/seq=%d/digest=%s", x.ReplicaId, x.View, x.SequenceNumber, x.BatchDigest)
}
