package persistence

import (
	"context"
	"testing"

	"github.com/bottlepay/lnmux/persistence/test"
	"github.com/bottlepay/lnmux/types"
	"github.com/go-pg/pg/v10"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestDB(t *testing.T) (*pg.DB, *PostgresPersister) {
	conn, dsn := test.ResetPGTestDB(t, &test.TestDBSettings{
		MigrationsPath: "./migrations",
	})

	log := zap.NewNop().Sugar()
	db, err := NewPostgresPersisterFromDSN(dsn, log)
	require.NoError(t, err)

	return conn, db
}

func TestSettleInvoice(t *testing.T) {
	pg, persister := setupTestDB(t)
	defer pg.Close()

	preimage := lntypes.Preimage{1}
	hash := preimage.Hash()

	// Initially no invoices are expected.
	_, _, err := persister.Get(context.Background(), hash)
	require.ErrorIs(t, err, types.ErrInvoiceNotFound)

	htlcs := map[types.CircuitKey]int64{
		{
			ChanID: 10,
			HtlcID: 11,
		}: 70,
		{
			ChanID: 11,
			HtlcID: 12,
		}: 30,
	}
	require.NoError(t, persister.RequestSettle(context.Background(), &InvoiceCreationData{
		InvoiceCreationData: types.InvoiceCreationData{
			PaymentPreimage: preimage,
			Value:           100,
			PaymentAddr:     [32]byte{2},
		},
	}, htlcs))

	_, err = persister.MarkHtlcSettled(context.Background(), hash, types.CircuitKey{
		ChanID: 99,
		HtlcID: 99,
	})
	require.ErrorIs(t, err, types.ErrHtlcNotFound)

	invoiceSettled, err := persister.MarkHtlcSettled(context.Background(), hash, types.CircuitKey{
		ChanID: 10,
		HtlcID: 11,
	})
	require.NoError(t, err)
	require.False(t, invoiceSettled)

	invoiceSettled, err = persister.MarkHtlcSettled(context.Background(), hash, types.CircuitKey{
		ChanID: 10,
		HtlcID: 11,
	})
	require.NoError(t, err)
	require.False(t, invoiceSettled)

	invoice, _, err := persister.Get(context.Background(), hash)
	require.NoError(t, err)
	require.False(t, invoice.Settled)

	invoiceSettled, err = persister.MarkHtlcSettled(context.Background(), hash, types.CircuitKey{
		ChanID: 11,
		HtlcID: 12,
	})
	require.NoError(t, err)
	require.True(t, invoiceSettled)

	invoice, htlcs, err = persister.Get(context.Background(), hash)
	require.NoError(t, err)
	require.Len(t, htlcs, 2)
	require.True(t, invoice.Settled)
}
