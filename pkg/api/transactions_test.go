package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestResolveTransactionMerchant(t *testing.T) {
	updated := "Updated merchant"
	empty := ""

	tests := []struct {
		name              string
		transactionType   models.TransactionDbType
		currentMerchant   string
		requestedMerchant *string
		expected          string
	}{
		{"old client preserves expense merchant", models.TRANSACTION_DB_TYPE_EXPENSE, "User merchant", nil, "User merchant"},
		{"explicit edit updates expense merchant", models.TRANSACTION_DB_TYPE_EXPENSE, "User merchant", &updated, "Updated merchant"},
		{"explicit edit clears expense merchant", models.TRANSACTION_DB_TYPE_EXPENSE, "User merchant", &empty, ""},
		{"income cannot carry merchant", models.TRANSACTION_DB_TYPE_INCOME, "User merchant", &updated, ""},
		{"transfer cannot carry merchant", models.TRANSACTION_DB_TYPE_TRANSFER_OUT, "User merchant", &updated, ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, resolveTransactionMerchant(test.transactionType, test.currentMerchant, test.requestedMerchant))
		})
	}
}

func TestCreateNewExpenseModelPreservesMerchant(t *testing.T) {
	transaction := Transactions.createNewTransactionModel(123, &models.TransactionCreateRequest{
		Type:     models.TRANSACTION_TYPE_EXPENSE,
		Merchant: "User-owned merchant",
	}, "127.0.0.1")

	assert.Equal(t, "User-owned merchant", transaction.Merchant)
}
