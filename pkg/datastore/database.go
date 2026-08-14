package datastore

import (
	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// Database represents a database instance
type Database struct {
	databaseType string
	engineGroup  *xorm.EngineGroup
}

// NewSession starts a new session with the specified context
func (db *Database) NewSession(c core.Context) *xorm.Session {
	return db.engineGroup.Context(NewXOrmContextAdapter(c))
}

// DoTransaction runs a new database transaction
func (db *Database) DoTransaction(c core.Context, fn func(sess *xorm.Session) error) (err error) {
	sess := db.engineGroup.NewSession()

	if c != nil {
		sess.Context(NewXOrmContextAdapter(c))
	}

	defer sess.Close()

	if err = sess.Begin(); err != nil {
		return err
	}

	if err = fn(sess); err != nil {
		_ = sess.Rollback()
		return err
	}

	if err = sess.Commit(); err != nil {
		return err
	}

	return nil
}

// SetSavePoint sets a save point in the current transaction for Postgres
func (db *Database) SetSavePoint(sess *xorm.Session, savePointName string) error {
	if db.databaseType == settings.PostgresDbType {
		_, err := sess.Exec("SAVEPOINT " + savePointName)
		return err
	}

	return nil
}

// RollbackToSavePoint rolls back to the specified save point in the current transaction for Postgres
func (db *Database) RollbackToSavePoint(sess *xorm.Session, savePointName string) error {
	if db.databaseType == settings.PostgresDbType {
		_, err := sess.Exec("ROLLBACK TO SAVEPOINT " + savePointName)
		return err
	}

	return nil
}

// SyncReconciliationConstraints creates PostgreSQL-only relational constraints
// after XORM has synchronized the participating tables.
func (db *Database) SyncReconciliationConstraints() error {
	if db.databaseType != settings.PostgresDbType {
		return nil
	}

	statements := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS uqe_transaction_observation_link_active ON transaction_observation_link (uid, observation_id) WHERE active = true`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uqe_financial_observation_uid_id ON financial_observation (uid, observation_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uqe_transaction_uid_id ON "transaction" (uid, transaction_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uqe_reconciliation_attempt_uid_id ON reconciliation_attempt (uid, attempt_id)`,
		`DO $$ BEGIN ALTER TABLE observation_external_ref ADD CONSTRAINT fk_observation_external_ref_observation FOREIGN KEY (uid, observation_id) REFERENCES financial_observation(uid, observation_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
		`DO $$ BEGIN ALTER TABLE transaction_observation_link ADD CONSTRAINT fk_transaction_observation_link_observation FOREIGN KEY (uid, observation_id) REFERENCES financial_observation(uid, observation_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
		`DO $$ BEGIN ALTER TABLE transaction_observation_link ADD CONSTRAINT fk_transaction_observation_link_transaction FOREIGN KEY (uid, transaction_id) REFERENCES "transaction"(uid, transaction_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
		`DO $$ BEGIN ALTER TABLE reconciliation_attempt ADD CONSTRAINT fk_reconciliation_attempt_observation FOREIGN KEY (uid, observation_id) REFERENCES financial_observation(uid, observation_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
		`DO $$ BEGIN ALTER TABLE reconciliation_review ADD CONSTRAINT fk_reconciliation_review_observation FOREIGN KEY (uid, observation_id) REFERENCES financial_observation(uid, observation_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
		`DO $$ BEGIN ALTER TABLE reconciliation_review ADD CONSTRAINT fk_reconciliation_review_attempt FOREIGN KEY (uid, attempt_id) REFERENCES reconciliation_attempt(uid, attempt_id); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	}

	for _, statement := range statements {
		if _, err := db.engineGroup.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}
