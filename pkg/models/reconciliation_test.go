package models

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReconciliationPersistenceSchema(t *testing.T) {
	assert.Equal(t, "financial_observation", (&FinancialObservation{}).TableName())
	assert.Equal(t, "observation_external_ref", (&ObservationExternalRef{}).TableName())
	assert.Equal(t, "transaction_observation_link", (&TransactionObservationLink{}).TableName())
	assert.Equal(t, "reconciliation_attempt", (&ReconciliationAttempt{}).TableName())
	assert.Equal(t, "reconciliation_review", (&ReconciliationReview{}).TableName())

	observationType := reflect.TypeOf(FinancialObservation{})
	var tags strings.Builder
	for i := 0; i < observationType.NumField(); i++ {
		field := observationType.Field(i)
		tags.WriteString(field.Tag.Get("xorm"))
		assert.NotContains(t, strings.ToLower(field.Name), "binary")
		assert.NotContains(t, strings.ToLower(field.Name), "bytes")
	}
	assert.Contains(t, tags.String(), "UQE_financial_observation_source_version")
	assert.Equal(t, reflect.Slice, reflect.TypeOf(FinancialObservation{}.RawPayload).Kind())

	reviewType := reflect.TypeOf(ReconciliationReview{})
	attemptField, ok := reviewType.FieldByName("AttemptId")
	assert.True(t, ok)
	assert.Contains(t, attemptField.Tag.Get("xorm"), "UNIQUE")
}

func TestReconciliationModelsAreUserScoped(t *testing.T) {
	models := []any{FinancialObservation{}, ObservationExternalRef{}, TransactionObservationLink{}, ReconciliationAttempt{}, ReconciliationReview{}}
	for _, model := range models {
		_, ok := reflect.TypeOf(model).FieldByName("Uid")
		assert.Truef(t, ok, "%T must be user scoped", model)
	}
}

func TestReceiptProvenanceContainsOnlyPictureReference(t *testing.T) {
	typeOfObservation := reflect.TypeOf(FinancialObservation{})
	receiptField, ok := typeOfObservation.FieldByName("ReceiptPictureId")
	assert.True(t, ok)
	assert.Equal(t, reflect.Int64, receiptField.Type.Kind())
	assert.Equal(t, "pending", FinancialObservationStatusPending)
	assert.Equal(t, "retrying", FinancialObservationStatusRetrying)
	assert.Equal(t, "review", FinancialObservationStatusReview)
}
