package services

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/storage"
)

func TestReconciliationReceiptPictureLifecycle(t *testing.T) {
	host := os.Getenv("EBK_TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("EBK_TEST_POSTGRES_HOST is not configured")
	}

	storageRoot := t.TempDir()
	config := &settings.Config{
		DatabaseConfig: &settings.DatabaseConfig{
			DatabaseType:     settings.PostgresDbType,
			DatabaseHost:     host,
			DatabaseName:     serviceTestEnvOrDefault("EBK_TEST_POSTGRES_DATABASE", "ezbookkeeping_reconciliation_test"),
			DatabaseUser:     serviceTestEnvOrDefault("EBK_TEST_POSTGRES_USER", "postgres"),
			DatabasePassword: os.Getenv("EBK_TEST_POSTGRES_PASSWORD"),
			DatabaseSSLMode:  "disable",
		},
		EnableTransactionPictures: true,
		StorageType:               settings.LocalFileSystemObjectStorageType,
		LocalFileSystemPath:       storageRoot,
	}

	require.NoError(t, datastore.InitializeDataStore(config))
	require.NoError(t, storage.InitializeStorageContainer(config))
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(
		new(models.Transaction),
		new(models.TransactionPictureInfo),
		new(models.FinancialObservation),
		new(models.ReconciliationAttempt),
		new(models.ReconciliationReview),
	))

	seed := time.Now().UnixNano() / 100
	uid := seed
	context := core.NewNullContext()

	t.Run("existing object", func(t *testing.T) {
		pictureID, observationID := seed+1, seed+2
		insertReceiptPictureFixture(t, context, uid, pictureID, observationID, models.FINANCIAL_OBSERVATION_STATUS_PENDING)
		saveReceiptObject(t, context, uid, pictureID)

		pictureInfo, err := TransactionPictures.ValidateReconciliationReceiptPicture(context, uid, observationID)
		require.NoError(t, err)
		assert.Equal(t, pictureID, pictureInfo.PictureId)
	})

	t.Run("missing object", func(t *testing.T) {
		pictureID, observationID := seed+3, seed+4
		insertReceiptPictureFixture(t, context, uid, pictureID, observationID, models.FINANCIAL_OBSERVATION_STATUS_PENDING)

		_, err := TransactionPictures.ValidateReconciliationReceiptPicture(context, uid, observationID)
		assert.True(t, errors.Is(err, errs.ErrTransactionPictureNoExists))
	})

	t.Run("retryable storage failure", func(t *testing.T) {
		failureUID := seed + 100
		pictureID, observationID := seed+5, seed+6
		insertReceiptPictureFixture(t, context, failureUID, pictureID, observationID, models.FINANCIAL_OBSERVATION_STATUS_PENDING)
		require.NoError(t, os.WriteFile(filepath.Join(storageRoot, "transaction", strconv.FormatInt(failureUID, 10)), []byte("not a directory"), 0o600))

		_, err := TransactionPictures.ValidateReconciliationReceiptPicture(context, failureUID, observationID)
		assert.Error(t, err)
		assert.False(t, errors.Is(err, errs.ErrTransactionPictureNoExists), "storage errors must be propagated for retry")
	})

	t.Run("cleanup protection", func(t *testing.T) {
		pictureID, observationID := seed+7, seed+8
		insertReceiptPictureFixture(t, context, uid, pictureID, observationID, models.FINANCIAL_OBSERVATION_STATUS_RETRYING)
		saveReceiptObject(t, context, uid, pictureID)

		err := TransactionPictures.RemoveUnusedTransactionPicture(context, uid, pictureID)
		assert.True(t, errors.Is(err, errs.ErrTransactionPictureInUse))

		_, err = datastore.Container.UserDataStore.Query(context, uid).ID(observationID).Cols("status").Update(&models.FinancialObservation{Status: models.FINANCIAL_OBSERVATION_STATUS_RECONCILED})
		require.NoError(t, err)
		require.NoError(t, TransactionPictures.RemoveUnusedTransactionPicture(context, uid, pictureID))
	})

	t.Run("open review cleanup protection", func(t *testing.T) {
		pictureID, observationID, attemptID := seed+10, seed+11, seed+12
		insertReceiptPictureFixture(t, context, uid, pictureID, observationID, models.FINANCIAL_OBSERVATION_STATUS_FAILED)
		saveReceiptObject(t, context, uid, pictureID)
		_, err := datastore.Container.UserDataStore.Query(context, uid).Insert(&models.ReconciliationAttempt{
			AttemptId: attemptID, Uid: uid, ObservationId: observationID, EngineVersion: "test",
			ScoringPolicyVersion: "test", Decision: "REVIEW", DecisionReason: "ambiguous",
			EvidenceSummary: json.RawMessage(`{}`), CreatedUnixTime: 1,
		})
		require.NoError(t, err)
		_, err = datastore.Container.UserDataStore.Query(context, uid).Insert(&models.ReconciliationReview{
			ReviewId: attemptID + 1, Uid: uid, ObservationId: observationID, AttemptId: attemptID,
			Status: models.RECONCILIATION_REVIEW_STATUS_OPEN, AlternativesSnapshot: json.RawMessage(`[]`),
			CreatedUnixTime: 1, UpdatedUnixTime: 1,
		})
		require.NoError(t, err)

		err = TransactionPictures.RemoveUnusedTransactionPicture(context, uid, pictureID)
		assert.True(t, errors.Is(err, errs.ErrTransactionPictureInUse))
	})

	for index, decision := range []string{"MATCH", "NEW"} {
		t.Run("object reuse on "+decision, func(t *testing.T) {
			pictureID := seed + int64(20+index*3)
			observationID := pictureID + 1
			transactionID := pictureID + 2
			insertReceiptPictureFixture(t, context, uid, pictureID, observationID, models.FINANCIAL_OBSERVATION_STATUS_PENDING)
			saveReceiptObject(t, context, uid, pictureID)
			insertExpenseFixture(t, context, uid, transactionID, transactionID)

			require.NoError(t, TransactionPictures.AttachReconciliationReceiptPicture(context, uid, observationID, transactionID))
			require.NoError(t, TransactionPictures.AttachReconciliationReceiptPicture(context, uid, observationID, transactionID), "attachment must be idempotent")

			pictureInfo, err := TransactionPictures.GetPictureInfoByPictureId(context, uid, pictureID)
			require.NoError(t, err)
			assert.Equal(t, transactionID, pictureInfo.TransactionId)

			content, err := TransactionPictures.GetPictureByPictureId(context, uid, pictureID, "jpg")
			require.NoError(t, err)
			assert.Equal(t, []byte("receipt-object"), content)
		})
	}
}

func TestRemoveUnusedTransactionPictureWithoutReconciliationTables(t *testing.T) {
	config := &settings.Config{DatabaseConfig: &settings.DatabaseConfig{
		DatabaseType: settings.Sqlite3DbType,
		DatabasePath: filepath.Join(t.TempDir(), "pictures.db"),
	}}
	require.NoError(t, datastore.InitializeDataStore(config))
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(new(models.TransactionPictureInfo)))

	context := core.NewNullContext()
	_, err := datastore.Container.UserDataStore.Query(context, 1).Insert(&models.TransactionPictureInfo{
		Uid: 1, PictureId: 2, TransactionId: models.TransactionPictureNewPictureTransactionId, PictureExtension: "jpg",
	})
	require.NoError(t, err)
	require.NoError(t, TransactionPictures.RemoveUnusedTransactionPicture(context, 1, 2))
}

func insertReceiptPictureFixture(t *testing.T, context core.Context, uid, pictureID, observationID int64, status string) {
	t.Helper()
	pictureInfo := &models.TransactionPictureInfo{
		Uid: uid, PictureId: pictureID, TransactionId: models.TransactionPictureNewPictureTransactionId,
		PictureExtension: "jpg", CreatedUnixTime: 1, UpdatedUnixTime: 1,
	}
	_, err := datastore.Container.UserDataStore.Query(context, uid).Insert(pictureInfo)
	require.NoError(t, err)

	observation := &models.FinancialObservation{
		ObservationId: observationID, Uid: uid, Source: models.RECONCILIATION_SOURCE_RECEIPT_OCR,
		SourceConnectionId: "receipt", SourceObservationId: strconv.FormatInt(observationID, 10), SourceVersion: "v1",
		ExpenseKind: "expense", SanitizedRawSnapshot: json.RawMessage(`{"total":100}`),
		NormalizedSnapshot: json.RawMessage(`{"amount":100}`), NormalizationVersion: "test",
		ReceiptPictureId: &pictureID, Status: status, ReceivedUnixTime: 1, CreatedUnixTime: 1, UpdatedUnixTime: 1,
	}
	_, err = datastore.Container.UserDataStore.Query(context, uid).Insert(observation)
	require.NoError(t, err)
}

func insertExpenseFixture(t *testing.T, context core.Context, uid, transactionID, transactionTime int64) {
	t.Helper()
	_, err := datastore.Container.UserDataStore.Query(context, uid).Insert(&models.Transaction{
		TransactionId: transactionID, Uid: uid, Type: models.TRANSACTION_DB_TYPE_EXPENSE,
		TransactionTime: transactionTime, Amount: 100,
	})
	require.NoError(t, err)
}

func saveReceiptObject(t *testing.T, context core.Context, uid, pictureID int64) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "receipt-object")
	require.NoError(t, err)
	defer file.Close()
	_, err = file.Write([]byte("receipt-object"))
	require.NoError(t, err)
	_, err = file.Seek(0, 0)
	require.NoError(t, err)
	require.NoError(t, TransactionPictures.SaveTransactionPicture(context, uid, pictureID, file, "jpg"))
}

func serviceTestEnvOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
