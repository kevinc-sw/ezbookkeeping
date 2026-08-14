package reconciliation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeReader struct {
	candidates []CanonicalCandidate
}

func (r fakeReader) FindCandidates(context.Context, NormalizedObservation) ([]CanonicalCandidate, bool, error) {
	return r.candidates, false, nil
}

func TestContractsWorkWithInMemoryFake(t *testing.T) {
	var reader CandidateReader = fakeReader{candidates: []CanonicalCandidate{{TransactionID: 42, UID: 7}}}
	candidates, overflow, err := reader.FindCandidates(context.Background(), NormalizedObservation{ObservationID: 1, UID: 7})
	assert.NoError(t, err)
	assert.False(t, overflow)
	assert.Equal(t, int64(42), candidates[0].TransactionID)
}

func TestAllFinancialResultsCarryAuditVersions(t *testing.T) {
	for _, decision := range []Decision{DecisionMatch, DecisionReview, DecisionNew} {
		result := Result{
			Decision:      decision,
			ReasonCode:    "test_reason",
			Evidence:      []Evidence{{Code: "test", Summary: "compact"}},
			PolicyVersion: PolicyVersionV1,
			EngineVersion: EngineVersionV1,
		}
		assert.NotEmpty(t, result.ReasonCode)
		assert.NotEmpty(t, result.Evidence)
		assert.NotEmpty(t, result.PolicyVersion)
		assert.NotEmpty(t, result.EngineVersion)
	}
}
