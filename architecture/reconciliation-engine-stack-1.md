# Reconciliation engine — Stack 1 implementation

Stack 1 implements the persistence and contract foundation described by the
reconciliation engine plan. It does not enable ingestion, matching, review
application, or automatic transaction mutation.

## Architecture decisions

- Reconciliation records use the existing `UserDataStore`, XORM models, ID
  generator, transaction helpers, and database maintenance command.
- PostgreSQL-only composite foreign keys and the active-link partial unique
  index are installed immediately after XORM table synchronization. No second
  ORM or migration framework is introduced.
- Receipt provenance stores only the existing transaction picture ID. Binary
  data stays behind `TransactionPictureService` and `StorageContainer`; receipt
  attachment changes picture metadata without copying the stored object.
- The pure `pkg/reconciliation` contracts use only standard-library types and
  read-only query interfaces. Persistence and transaction mutation remain in
  application services.
- Observation persistence follows existing UID-filtered service queries.
  Snapshots are validated at the persistence boundary to reject credential and
  binary-content keys.

## Schema

The stack adds `financial_observation`, `observation_external_ref`,
`transaction_observation_link`, `reconciliation_attempt`, and
`reconciliation_review`. It adds `merchant` and `merchant_user_owned` to the
existing transaction table. There is deliberately no persisted candidate table.

## Deferred work

Normalization, candidate selection, deterministic matching, scoring, decision
policy, ingestion orchestration, atomic apply, retries, review APIs, and rollout
flags remain assigned to later REC tasks. Automatic matching remains disabled.
