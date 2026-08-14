package services

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

func TestReconciliationObservationRepository(t *testing.T) {
	host := os.Getenv("EBK_TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("EBK_TEST_POSTGRES_HOST is not configured")
	}

	initializeReconciliationRepositoryTestDatabase(t, host)

	context := core.NewNullContext()
	seed := time.Now().UnixNano() / 100
	uid, otherUID := seed, seed+1
	observationID := seed + 10
	identity := strconv.FormatInt(seed, 10)
	original := newRepositoryObservation(uid, observationID, identity, "v1", nil, `{"merchant":"Corner Shop"}`)

	stored, created, err := ReconciliationObservations.CreateOrGetObservation(context, uid, original)
	require.NoError(t, err)
	require.True(t, created)
	assert.Equal(t, observationID, stored.ObservationId)

	redelivery := newRepositoryObservation(uid, observationID+1, identity, "v1", nil, `{"merchant":"changed input"}`)
	storedAgain, created, err := ReconciliationObservations.CreateOrGetObservation(context, uid, redelivery)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, observationID, storedAgain.ObservationId)
	assert.JSONEq(t, `{"merchant":"Corner Shop"}`, string(storedAgain.SanitizedRawSnapshot), "source versions are immutable")

	supersedes := observationID
	newVersion := newRepositoryObservation(uid, observationID+2, identity, "v2", &supersedes, `{"merchant":"Corner Shop","state":"posted"}`)
	storedVersion, created, err := ReconciliationObservations.CreateOrGetObservation(context, uid, newVersion)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, observationID+2, storedVersion.ObservationId)

	_, err = ReconciliationObservations.GetObservation(context, otherUID, observationID)
	assert.True(t, errors.Is(err, errs.ErrReconciliationObservationNotFound))

	raw, normalized, err := ReconciliationObservations.LoadObservationSnapshots(context, uid, observationID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"merchant":"Corner Shop"}`, string(raw))
	assert.JSONEq(t, `{"amount":1200,"currency":"CAD"}`, string(normalized))

	reference := &models.ObservationExternalRef{ExternalRefId: seed + 20, ObservationId: observationID, Namespace: "bank.transaction", Value: "masked-reference", RelationType: "source", CreatedUnixTime: 1}
	storedReference, created, err := ReconciliationObservations.AddExternalReference(context, uid, reference)
	require.NoError(t, err)
	assert.True(t, created)
	_, created, err = ReconciliationObservations.AddExternalReference(context, uid, &models.ObservationExternalRef{ExternalRefId: seed + 21, ObservationId: observationID, Namespace: reference.Namespace, Value: reference.Value, RelationType: reference.RelationType})
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, storedReference.ExternalRefId, reference.ExternalRefId)

	require.NoError(t, ReconciliationObservations.UpdateProcessingState(context, uid, observationID, ObservationProcessingState{Status: models.FINANCIAL_OBSERVATION_STATUS_RETRYING, RetryCount: 2, NextRetryUnixTime: 100, LastErrorClass: "storage_unavailable"}))
	updated, err := ReconciliationObservations.GetObservation(context, uid, observationID)
	require.NoError(t, err)
	assert.Equal(t, uint8(2), updated.RetryCount)
	assert.JSONEq(t, `{"merchant":"Corner Shop"}`, string(updated.SanitizedRawSnapshot), "processing updates cannot mutate immutable snapshots")

	attempt := &models.ReconciliationAttempt{AttemptId: seed + 30, ObservationId: observationID, EngineVersion: "engine-v1", ScoringPolicyVersion: "policy-v1", Decision: "REVIEW", DecisionReason: "ambiguous", EvidenceSummary: json.RawMessage(`{"codes":["amount_exact"]}`)}
	require.NoError(t, ReconciliationObservations.AddAttempt(context, uid, attempt))

	transactionID := seed + 40
	_, err = datastore.Container.UserDataStore.Query(context, uid).Insert(&models.Transaction{TransactionId: transactionID, Uid: uid, Type: models.TRANSACTION_DB_TYPE_EXPENSE, TransactionTime: seed, Amount: 1200})
	require.NoError(t, err)
	link := &models.TransactionObservationLink{LinkId: seed + 41, ObservationId: observationID, TransactionId: transactionID, Active: true, LinkReason: "user_confirmed", Actor: "user", AttemptId: &attempt.AttemptId}
	require.NoError(t, ReconciliationObservations.AddLink(context, uid, link))
	require.NoError(t, ReconciliationObservations.RevokeActiveLink(context, uid, observationID, "superseded"))
	link.LinkId = seed + 42
	link.Active = true
	link.LinkReason = "reconciled"
	require.NoError(t, ReconciliationObservations.AddLink(context, uid, link))
	links, err := ReconciliationObservations.GetLinks(context, uid, observationID)
	require.NoError(t, err)
	require.Len(t, links, 2)
	assert.False(t, links[0].Active)
	assert.True(t, links[1].Active)

	review := &models.ReconciliationReview{ReviewId: seed + 50, ObservationId: observationID, AttemptId: attempt.AttemptId, Status: models.RECONCILIATION_REVIEW_STATUS_OPEN, RecommendedTransactionId: &transactionID, AlternativesSnapshot: json.RawMessage(`[{"transactionId":"masked"}]`)}
	require.NoError(t, ReconciliationObservations.AddReview(context, uid, review))
	attempts, err := ReconciliationObservations.GetAttempts(context, uid, observationID)
	require.NoError(t, err)
	reviews, err := ReconciliationObservations.GetReviews(context, uid, observationID)
	require.NoError(t, err)
	assert.Len(t, attempts, 1)
	assert.Len(t, reviews, 1)

	otherReferences, err := ReconciliationObservations.GetExternalReferences(context, otherUID, observationID)
	require.NoError(t, err)
	assert.Empty(t, otherReferences)
}

func TestReconciliationObservationInsertIsConcurrentAndIdempotent(t *testing.T) {
	if os.Getenv("EBK_TEST_POSTGRES_HOST") == "" {
		t.Skip("EBK_TEST_POSTGRES_HOST is not configured")
	}
	initializeReconciliationRepositoryTestDatabase(t, os.Getenv("EBK_TEST_POSTGRES_HOST"))

	seed := time.Now().UnixNano() / 100
	uid := seed
	identity := strconv.FormatInt(seed, 10)
	context := core.NewNullContext()
	const workers = 6

	ids := make(chan int64, workers)
	errorsFound := make(chan error, workers)
	var waitGroup sync.WaitGroup
	for index := 0; index < workers; index++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			observation := newRepositoryObservation(uid, seed+int64(index)+10, identity, "concurrent-v1", nil, `{"sanitized":true}`)
			stored, _, err := ReconciliationObservations.CreateOrGetObservation(context, uid, observation)
			if err != nil {
				errorsFound <- err
				return
			}
			ids <- stored.ObservationId
		}(index)
	}
	waitGroup.Wait()
	close(ids)
	close(errorsFound)

	for err := range errorsFound {
		require.NoError(t, err)
	}
	var first int64
	for id := range ids {
		if first == 0 {
			first = id
		}
		assert.Equal(t, first, id)
	}
}

func newRepositoryObservation(uid, observationID int64, identity, version string, supersedes *int64, raw string) *models.FinancialObservation {
	return &models.FinancialObservation{
		ObservationId: observationID, Uid: uid, Source: models.RECONCILIATION_SOURCE_PLAID,
		SourceConnectionId: "connection", SourceObservationId: identity, SourceVersion: version,
		ExpenseKind: "expense", SanitizedRawSnapshot: json.RawMessage(raw),
		NormalizedSnapshot: json.RawMessage(`{"amount":1200,"currency":"CAD"}`), NormalizationVersion: "normalization-v1",
		Status: models.FINANCIAL_OBSERVATION_STATUS_PENDING, SupersedesObservationId: supersedes,
	}
}

func initializeReconciliationRepositoryTestDatabase(t *testing.T, host string) {
	t.Helper()
	config := &settings.Config{DatabaseConfig: &settings.DatabaseConfig{
		DatabaseType:     settings.PostgresDbType,
		DatabaseHost:     host,
		DatabaseName:     serviceTestEnvOrDefault("EBK_TEST_POSTGRES_DATABASE", "ezbookkeeping_reconciliation_test"),
		DatabaseUser:     serviceTestEnvOrDefault("EBK_TEST_POSTGRES_USER", "postgres"),
		DatabasePassword: os.Getenv("EBK_TEST_POSTGRES_PASSWORD"),
		DatabaseSSLMode:  "disable",
	}}
	require.NoError(t, datastore.InitializeDataStore(config))
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(
		new(models.Transaction),
		new(models.FinancialObservation),
		new(models.ObservationExternalRef),
		new(models.TransactionObservationLink),
		new(models.ReconciliationAttempt),
		new(models.ReconciliationReview),
	))
}
