package services

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateObservationSnapshot(t *testing.T) {
	assert.NoError(t, validateObservationSnapshot(json.RawMessage(`{"merchant":"Store","picture_id":"123"}`)))
	for _, payload := range []string{
		`{"access_token":"secret"}`,
		`{"nested":{"password":"secret"}}`,
		`{"file_bytes":"AAEC"}`,
		`{"imageBytes":"AAEC"}`,
	} {
		assert.ErrorIs(t, validateObservationSnapshot(json.RawMessage(payload)), ErrUnsafeObservationSnapshot)
	}
}

func TestValidateObservationSnapshotRejectsMalformedJSON(t *testing.T) {
	assert.Error(t, validateObservationSnapshot(json.RawMessage(`{"merchant"`)))
}
