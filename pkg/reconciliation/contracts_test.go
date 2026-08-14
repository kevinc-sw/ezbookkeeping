package reconciliation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryReader struct {
	candidates []Candidate
	linked     map[int64]Candidate
	references map[ExternalReference][]Candidate
}

func (r *memoryReader) FindCandidates(_ context.Context, query CandidateQuery) ([]Candidate, error) {
	limit := query.Limit
	if limit <= 0 || limit > len(r.candidates) {
		limit = len(r.candidates)
	}
	return append([]Candidate(nil), r.candidates[:limit]...), nil
}

func (r *memoryReader) FindTransactionByID(_ context.Context, uid int64, transactionID int64) (*Candidate, error) {
	for i := range r.candidates {
		if r.candidates[i].UID == uid && r.candidates[i].TransactionID == transactionID {
			candidate := r.candidates[i]
			return &candidate, nil
		}
	}
	return nil, nil
}

func (r *memoryReader) FindLinkedTransaction(_ context.Context, uid int64, observationID int64) (*Candidate, error) {
	candidate, exists := r.linked[observationID]
	if !exists || candidate.UID != uid {
		return nil, nil
	}
	return &candidate, nil
}

func (r *memoryReader) FindTransactionsByExternalReference(_ context.Context, uid int64, reference ExternalReference) ([]Candidate, error) {
	var matches []Candidate
	for _, candidate := range r.references[reference] {
		if candidate.UID == uid {
			matches = append(matches, candidate)
		}
	}
	return matches, nil
}

var _ CandidateReader = (*memoryReader)(nil)
var _ DeterministicReader = (*memoryReader)(nil)

func TestContractsSupportPureInMemoryReaders(t *testing.T) {
	ref := ExternalReference{Namespace: "bank.transaction", Value: "masked-id"}
	reader := &memoryReader{
		candidates: []Candidate{{TransactionID: 10, UID: 1}, {TransactionID: 20, UID: 2}},
		linked:     map[int64]Candidate{100: {TransactionID: 10, UID: 1}},
		references: map[ExternalReference][]Candidate{ref: {{TransactionID: 10, UID: 1}, {TransactionID: 20, UID: 2}}},
	}

	candidates, err := reader.FindCandidates(context.Background(), CandidateQuery{Observation: NormalizedObservation{UID: 1}, Limit: 1})
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	assert.Equal(t, int64(10), candidates[0].TransactionID)

	linked, err := reader.FindLinkedTransaction(context.Background(), 1, 100)
	require.NoError(t, err)
	require.NotNil(t, linked)
	assert.Equal(t, int64(10), linked.TransactionID)

	matches, err := reader.FindTransactionsByExternalReference(context.Background(), 1, ref)
	require.NoError(t, err)
	require.Len(t, matches, 1, "the fake demonstrates required UID isolation")
}

func TestEveryDecisionCarriesVersionedCompactEvidence(t *testing.T) {
	for _, decision := range []Decision{DecisionMatch, DecisionReview, DecisionNew} {
		result := Result{
			Decision:      decision,
			ReasonCode:    "test_reason",
			Evidence:      []Evidence{{Code: "amount_exact", MaskedValue: "same amount"}},
			PolicyVersion: "policy-v1",
			EngineVersion: "engine-v1",
		}

		assert.NotEmpty(t, result.ReasonCode)
		assert.NotEmpty(t, result.Evidence)
		assert.NotEmpty(t, result.PolicyVersion)
		assert.NotEmpty(t, result.EngineVersion)
	}
}
