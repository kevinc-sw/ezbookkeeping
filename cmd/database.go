package cmd

import (
	"github.com/urfave/cli/v3"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// Database represents the database command
var Database = &cli.Command{
	Name:  "database",
	Usage: "ezBookkeeping database maintenance",
	Commands: []*cli.Command{
		{
			Name:   "update",
			Usage:  "Update database structure",
			Action: bindAction(updateDatabaseStructure),
		},
	},
}

func updateDatabaseStructure(c *core.CliContext) error {
	_, err := initializeSystem(c)

	if err != nil {
		return err
	}

	log.CliInfof(c, "[database.updateDatabaseStructure] starting maintaining")

	err = updateAllDatabaseTablesStructure(c)

	if err != nil {
		log.CliErrorf(c, "[database.updateDatabaseStructure] update database table structure failed, because %s", err.Error())
		return err
	}

	log.CliInfof(c, "[database.updateDatabaseStructure] all tables maintained successfully")
	return nil
}

func updateAllDatabaseTablesStructure(c *core.CliContext) error {
	var err error

	err = datastore.Container.UserStore.SyncStructs(new(models.User))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] user table maintained successfully")

	err = datastore.Container.UserStore.SyncStructs(new(models.TwoFactor))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] two-factor table maintained successfully")

	err = datastore.Container.UserStore.SyncStructs(new(models.TwoFactorRecoveryCode))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] two-factor recovery code table maintained successfully")

	err = datastore.Container.TokenStore.SyncStructs(new(models.TokenRecord))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] token record table maintained successfully")

	err = datastore.Container.UserDataStore.SyncStructs(new(models.Account))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] account table maintained successfully")

	err = datastore.Container.UserDataStore.SyncStructs(new(models.Transaction))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] transaction table maintained successfully")

	err = datastore.Container.UserDataStore.SyncStructs(new(models.TransactionCategory))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] transaction category table maintained successfully")

	err = datastore.Container.UserDataStore.SyncStructs(new(models.TransactionTagGroup))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] transaction tag group table maintained successfully")

	err = datastore.Container.UserDataStore.SyncStructs(new(models.TransactionTag))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] transaction tag table maintained successfully")

	err = datastore.Container.UserDataStore.SyncStructs(new(models.TransactionTagIndex))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] transaction tag index table maintained successfully")

	err = datastore.Container.UserDataStore.SyncStructs(new(models.TransactionTemplate))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] transaction template table maintained successfully")

	err = datastore.Container.UserDataStore.SyncStructs(new(models.TransactionPictureInfo))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] transaction picture table maintained successfully")

	err = updateReconciliationDatabaseTablesStructure(c)

	if err != nil {
		return err
	}

	err = datastore.Container.UserDataStore.SyncStructs(new(models.UserCustomExchangeRate))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] user custom exchange rate table maintained successfully")

	err = datastore.Container.UserDataStore.SyncStructs(new(models.UserApplicationCloudSetting))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] user application cloud settings table maintained successfully")

	err = datastore.Container.UserDataStore.SyncStructs(new(models.UserExternalAuth))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] user external auth table maintained successfully")

	err = datastore.Container.UserDataStore.SyncStructs(new(models.InsightsExplorer))

	if err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateAllDatabaseTablesStructure] insights explorer table maintained successfully")

	return nil
}

func updateReconciliationDatabaseTablesStructure(c core.Context) error {
	database := datastore.Container.UserDataStore.Choose(0)

	if !database.IsPostgres() {
		return nil
	}

	err := datastore.Container.UserDataStore.SyncStructs(
		new(models.FinancialObservation),
		new(models.ObservationExternalRef),
		new(models.TransactionObservationLink),
		new(models.ReconciliationAttempt),
		new(models.ReconciliationReview),
	)

	if err != nil {
		return err
	}

	if err = ensureReconciliationDatabaseConstraints(c, database); err != nil {
		return err
	}

	log.BootInfof(c, "[database.updateReconciliationDatabaseTablesStructure] reconciliation tables maintained successfully")
	return nil
}

func ensureReconciliationDatabaseConstraints(c core.Context, database *datastore.Database) error {
	sess := database.NewSession(c)
	defer sess.Close()

	statements := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS UQE_transaction_observation_link_active ON transaction_observation_link (observation_id) WHERE active = TRUE",
		"CREATE INDEX IF NOT EXISTS IDX_transaction_observation_link_transaction_id ON transaction_observation_link (transaction_id)",
		"CREATE INDEX IF NOT EXISTS IDX_financial_observation_receipt_picture_id ON financial_observation (receipt_picture_id) WHERE receipt_picture_id IS NOT NULL",
		"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_financial_observation_receipt_picture') THEN ALTER TABLE financial_observation ADD CONSTRAINT fk_financial_observation_receipt_picture FOREIGN KEY (uid, receipt_picture_id) REFERENCES transaction_picture_info(uid, picture_id); END IF; END $$",
		"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_financial_observation_supersedes') THEN ALTER TABLE financial_observation ADD CONSTRAINT fk_financial_observation_supersedes FOREIGN KEY (uid, supersedes_observation_id) REFERENCES financial_observation(uid, observation_id); END IF; END $$",
		"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_observation_external_ref_observation') THEN ALTER TABLE observation_external_ref ADD CONSTRAINT fk_observation_external_ref_observation FOREIGN KEY (uid, observation_id) REFERENCES financial_observation(uid, observation_id); END IF; END $$",
		"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_transaction_observation_link_observation') THEN ALTER TABLE transaction_observation_link ADD CONSTRAINT fk_transaction_observation_link_observation FOREIGN KEY (uid, observation_id) REFERENCES financial_observation(uid, observation_id); END IF; END $$",
		"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_transaction_observation_link_transaction') THEN ALTER TABLE transaction_observation_link ADD CONSTRAINT fk_transaction_observation_link_transaction FOREIGN KEY (uid, transaction_id) REFERENCES transaction(uid, transaction_id); END IF; END $$",
		"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_transaction_observation_link_attempt') THEN ALTER TABLE transaction_observation_link ADD CONSTRAINT fk_transaction_observation_link_attempt FOREIGN KEY (uid, attempt_id) REFERENCES reconciliation_attempt(uid, attempt_id); END IF; END $$",
		"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_reconciliation_attempt_observation') THEN ALTER TABLE reconciliation_attempt ADD CONSTRAINT fk_reconciliation_attempt_observation FOREIGN KEY (uid, observation_id) REFERENCES financial_observation(uid, observation_id); END IF; END $$",
		"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_reconciliation_attempt_target') THEN ALTER TABLE reconciliation_attempt ADD CONSTRAINT fk_reconciliation_attempt_target FOREIGN KEY (uid, target_transaction_id) REFERENCES transaction(uid, transaction_id); END IF; END $$",
		"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_reconciliation_review_observation') THEN ALTER TABLE reconciliation_review ADD CONSTRAINT fk_reconciliation_review_observation FOREIGN KEY (uid, observation_id) REFERENCES financial_observation(uid, observation_id); END IF; END $$",
		"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_reconciliation_review_attempt') THEN ALTER TABLE reconciliation_review ADD CONSTRAINT fk_reconciliation_review_attempt FOREIGN KEY (uid, attempt_id) REFERENCES reconciliation_attempt(uid, attempt_id); END IF; END $$",
		"DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_reconciliation_review_recommended') THEN ALTER TABLE reconciliation_review ADD CONSTRAINT fk_reconciliation_review_recommended FOREIGN KEY (uid, recommended_transaction_id) REFERENCES transaction(uid, transaction_id); END IF; END $$",
	}

	for _, statement := range statements {
		if _, err := sess.Exec(statement); err != nil {
			return err
		}
	}

	return nil
}
