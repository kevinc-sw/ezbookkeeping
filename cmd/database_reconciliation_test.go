package cmd

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

func TestReconciliationPostgresSchema(t *testing.T) {
	host := os.Getenv("EBK_TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("EBK_TEST_POSTGRES_HOST is not configured")
	}

	config := &settings.Config{
		DatabaseConfig: &settings.DatabaseConfig{
			DatabaseType:     settings.PostgresDbType,
			DatabaseHost:     host,
			DatabaseName:     getenvOrDefault("EBK_TEST_POSTGRES_DATABASE", "ezbookkeeping_reconciliation_test"),
			DatabaseUser:     getenvOrDefault("EBK_TEST_POSTGRES_USER", "postgres"),
			DatabasePassword: os.Getenv("EBK_TEST_POSTGRES_PASSWORD"),
			DatabaseSSLMode:  "disable",
		},
	}

	require.NoError(t, datastore.InitializeDataStore(config))
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(new(models.Transaction), new(models.TransactionPictureInfo)))
	require.NoError(t, updateReconciliationDatabaseTablesStructure(core.NewNullContext()))
	require.NoError(t, updateReconciliationDatabaseTablesStructure(core.NewNullContext()), "schema updates must be idempotent")

	db := datastore.Container.UserDataStore.Choose(0)
	sess := db.NewSession(core.NewNullContext())
	defer sess.Close()

	transaction := newTestExpense(1001, 2001, 1700000000000)
	_, err := sess.Insert(transaction)
	require.NoError(t, err)

	observation := newTestObservation(3001, 1001, "source-id", "v1")
	_, err = sess.Insert(observation)
	require.NoError(t, err)

	duplicate := newTestObservation(3002, 1001, "source-id", "v1")
	_, err = sess.Insert(duplicate)
	assert.Error(t, err, "source/version delivery must be idempotent")

	otherUserObservation := newTestObservation(3003, 1002, "source-id", "v1")
	_, err = sess.Insert(otherUserObservation)
	require.NoError(t, err, "source identity uniqueness must be user scoped")

	_, err = sess.Insert(&models.TransactionObservationLink{
		LinkId: 4001, Uid: 1002, ObservationId: 3001, TransactionId: 2001,
		Active: true, LinkReason: "test", Actor: "system", CreatedUnixTime: 1,
	})
	assert.Error(t, err, "cross-user observation links must fail")

	_, err = sess.Insert(&models.TransactionObservationLink{
		LinkId: 4002, Uid: 1001, ObservationId: 3001, TransactionId: 2001,
		Active: true, LinkReason: "test", Actor: "system", CreatedUnixTime: 1,
	})
	require.NoError(t, err)

	_, err = sess.Insert(&models.TransactionObservationLink{
		LinkId: 4003, Uid: 1001, ObservationId: 3001, TransactionId: 2001,
		Active: true, LinkReason: "duplicate", Actor: "system", CreatedUnixTime: 2,
	})
	assert.Error(t, err, "an observation must have at most one active link")

	_, err = sess.Insert(&models.TransactionObservationLink{
		LinkId: 4004, Uid: 1001, ObservationId: 3001, TransactionId: 2001,
		Active: false, LinkReason: "history", Actor: "system", CreatedUnixTime: 2,
	})
	require.NoError(t, err, "historical links must coexist with the active link")

	attempt := &models.ReconciliationAttempt{
		AttemptId: 5001, Uid: 1001, ObservationId: 3001, EngineVersion: "test",
		ScoringPolicyVersion: "test", Decision: "REVIEW", DecisionReason: "ambiguous",
		EvidenceSummary: json.RawMessage(`{"candidateCount":2}`), CreatedUnixTime: 3,
	}
	_, err = sess.Insert(attempt)
	require.NoError(t, err)

	_, err = sess.Insert(&models.ReconciliationReview{
		ReviewId: 6001, Uid: 1001, ObservationId: 3001, AttemptId: 5001, Status: "open",
		AlternativesSnapshot: json.RawMessage(`[{"transactionId":"2001"}]`), CreatedUnixTime: 4, UpdatedUnixTime: 4,
	})
	require.NoError(t, err, "reviews must relate to an existing attempt and observation")

	var jsonbColumns int64
	has, err := sess.SQL("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('financial_observation','reconciliation_attempt','reconciliation_review') AND data_type='jsonb'").Get(&jsonbColumns)
	require.True(t, has)
	require.NoError(t, err)
	assert.EqualValues(t, 4, jsonbColumns)

	var forbiddenTables int64
	has, err = sess.SQL("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name LIKE '%candidate%'").Get(&forbiddenTables)
	require.True(t, has)
	require.NoError(t, err)
	assert.Zero(t, forbiddenTables)

	var binaryColumns int64
	has, err = sess.SQL("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name IN ('financial_observation','observation_external_ref','transaction_observation_link','reconciliation_attempt','reconciliation_review') AND data_type='bytea'").Get(&binaryColumns)
	require.True(t, has)
	require.NoError(t, err)
	assert.Zero(t, binaryColumns)
}

func newTestObservation(id, uid int64, sourceID, version string) *models.FinancialObservation {
	return &models.FinancialObservation{
		ObservationId: id, Uid: uid, Source: models.RECONCILIATION_SOURCE_PLAID,
		SourceConnectionId: "connection", SourceObservationId: sourceID, SourceVersion: version,
		ExpenseKind: "expense", SanitizedRawSnapshot: json.RawMessage(`{"amount":100}`),
		NormalizedSnapshot: json.RawMessage(`{"amount":100}`), NormalizationVersion: "test",
		Status: "pending", ReceivedUnixTime: 1, CreatedUnixTime: 1, UpdatedUnixTime: 1,
	}
}

func newTestExpense(uid, transactionID, transactionTime int64) *models.Transaction {
	return &models.Transaction{
		TransactionId: transactionID, Uid: uid, Type: models.TRANSACTION_DB_TYPE_EXPENSE,
		TransactionTime: transactionTime, Amount: 100, Comment: "existing expense",
	}
}

func getenvOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
