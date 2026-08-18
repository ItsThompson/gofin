package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/internal/model"
)

func TestListCurrencies_Success(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/currencies", "user-123", nil)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.CurrencyListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Currencies)

	byCode := make(map[string]model.CurrencyData, len(resp.Currencies))
	for _, currency := range resp.Currencies {
		byCode[currency.Code] = currency
	}

	usd, ok := byCode["USD"]
	require.True(t, ok, "USD missing from response")
	assert.Equal(t, "$", usd.Symbol)
	assert.Equal(t, "US Dollar", usd.Name)
	assert.Equal(t, 2, usd.MinorUnitDigits)

	jpy, ok := byCode["JPY"]
	require.True(t, ok, "JPY missing from response")
	assert.Equal(t, 0, jpy.MinorUnitDigits)
}

func TestListCurrencies_RequiresUserID(t *testing.T) {
	repo := new(mockFinanceRepository)
	txBeginner := new(mockTxBeginner)
	r := setupTestRouter(repo, txBeginner)

	w := doJSONWithUserID(r, "GET", "/api/finance/currencies", "", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
