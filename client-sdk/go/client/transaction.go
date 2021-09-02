package client

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

// TransactionBuilder is a helper for building and submitting transactions.
type TransactionBuilder struct {
	rc RuntimeClient
	tx *types.Transaction
	ts *types.TransactionSigner

	callMeta interface{}
}

// NewTransactionBuilder creates a new transaction builder.
func NewTransactionBuilder(rc RuntimeClient, method string, body interface{}) *TransactionBuilder {
	return &TransactionBuilder{
		rc: rc,
		tx: types.NewTransaction(nil, method, body),
	}
}

// SetFeeAmount configures the fee amount to be paid by the caller.
func (tb *TransactionBuilder) SetFeeAmount(amount types.BaseUnits) *TransactionBuilder {
	tb.tx.AuthInfo.Fee.Amount = amount
	return tb
}

// SetFeeGas configures the maximum gas amount that can be used by the transaction.
func (tb *TransactionBuilder) SetFeeGas(gas uint64) *TransactionBuilder {
	tb.tx.AuthInfo.Fee.Gas = gas
	return tb
}

// SetFeeConsensusMessages configures the maximum number of consensus messages that can be emitted
// by the transaction.
func (tb *TransactionBuilder) SetFeeConsensusMessages(consensusMessages uint32) *TransactionBuilder {
	tb.tx.AuthInfo.Fee.ConsensusMessages = consensusMessages
	return tb
}

// SetCallFormat changes the transaction's call format.
//
// Depending on the call format this operation my require queries into the runtime in order to
// retrieve the required parameters.
//
// This method can only be called as long as the current call format is CallFormatPlain and will
// fail otherwise.
func (tb *TransactionBuilder) SetCallFormat(ctx context.Context, format types.CallFormat) error {
	if tb.tx.Call.Format != types.CallFormatPlain || tb.callMeta != nil {
		return fmt.Errorf("can only change call format from CallFormatPlain")
	}

	tb.tx.Call.Format = format
	encodedCall, meta, err := tb.encodeCall(ctx, &tb.tx.Call)
	if err != nil {
		return err
	}
	tb.tx.Call = *encodedCall
	tb.callMeta = meta
	return nil
}

// AppendAuthSignature appends a new transaction signer information with a signature address
// specification to the transaction.
func (tb *TransactionBuilder) AppendAuthSignature(pk signature.PublicKey, nonce uint64) *TransactionBuilder {
	tb.tx.AppendAuthSignature(pk, nonce)
	return tb
}

// AppendAuthMultisig appends a new transaction signer information with a multisig address
// specification to the transaction.
func (tb *TransactionBuilder) AppendAuthMultisig(config *types.MultisigConfig, nonce uint64) *TransactionBuilder {
	tb.tx.AppendAuthMultisig(config, nonce)
	return tb
}

// GetTransaction returns the underlying unsigned transaction.
func (tb *TransactionBuilder) GetTransaction() *types.Transaction {
	return tb.tx
}

// AppendSign signs the transaction and appends the signature.
//
// The signer must be specified in the AuthInfo.
func (tb *TransactionBuilder) AppendSign(ctx context.Context, signer signature.Signer) error {
	if tb.ts == nil {
		tb.ts = tb.tx.PrepareForSigning()
	}
	rtInfo, err := tb.rc.GetInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve runtime info: %w", err)
	}
	return tb.ts.AppendSign(rtInfo.ChainContext, signer)
}

// SubmitTx submits a transaction to the runtime transaction scheduler and waits for transaction
// execution results.
func (tb *TransactionBuilder) SubmitTx(ctx context.Context, rsp interface{}) error {
	if tb.ts == nil {
		return fmt.Errorf("unable to submit unsigned transaction")
	}

	result, err := tb.rc.SubmitTxRaw(ctx, tb.ts.UnverifiedTransaction())
	if err != nil {
		return err
	}
	result, err = tb.decodeResult(result, tb.callMeta)
	if err != nil {
		return err
	}

	switch {
	case result.IsUnknown():
		// This should never happen as the inner result should not be unknown.
		return fmt.Errorf("got unknown result: %X", result.Unknown)
	case result.IsSuccess():
		if rsp != nil {
			if err := cbor.Unmarshal(result.Ok, rsp); err != nil {
				return fmt.Errorf("failed to unmarshal call result: %w", err)
			}
		}
		return nil
	default:
		return result.Failed
	}
}

// SubmitTxNoWait submits a transaction to the runtime transaction scheduler but does not wait for
// transaction execution.
func (tb *TransactionBuilder) SubmitTxNoWait(ctx context.Context) error {
	if tb.ts == nil {
		return fmt.Errorf("unable to submit unsigned transaction")
	}
	return tb.rc.SubmitTxNoWait(ctx, tb.ts.UnverifiedTransaction())
}
