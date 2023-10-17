package lspclient

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/btcsuite/btcd/chaincfg"
	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
)

func getChainParams(network string) *chaincfg.Params {
	switch network {
	case "mainnet":
		return &chaincfg.MainNetParams
	case "testnet":
		return &chaincfg.TestNet3Params
	case "signet":
		return &chaincfg.SigNetParams
	default:
		return nil
	}
}

func paymentFee(chanInfo *lspdrpc.ChannelInformationReply, requestAmountSat int64) int64 {
	// calculate server fee
	feeMsat := requestAmountSat * 1000 * chanInfo.ChannelFeePermyriad / 10000
	if feeMsat < chanInfo.ChannelMinimumFeeMsat {
		feeMsat = chanInfo.ChannelMinimumFeeMsat
	}
	return feeMsat / 1000
}

func hubHopFeeMsat(chanInfo *lspdrpc.ChannelInformationReply, amountMsat int64) int64 {
	return int64(chanInfo.BaseFeeMsat + int64(float64(amountMsat)*chanInfo.FeeRate))
}

func receivableMax(lndApi LndApiConnector) (int64, error) {
	const marginRate int64 = 50

	listRes, err := lndApi.ListChannels()
	if err != nil {
		return 0, err
	}

	var maxReceivable int64
	for _, v := range listRes.Channels {
		// RemoteBalance = Capacity - LocalBalance - CommitFee - Anchorx2
		// Receivable = RemoteBalance - ChanReserveSat - margin
		//
		// The receivable amount should have a margin to allow for possible
		// changes due to BOLT "update_fee" message.
		//
		// NOTE: should use bandwidth?
		margin := int64(v.CommitFee * marginRate / 100)
		recv := v.RemoteBalance - int64(v.RemoteConstraints.ChanReserveSat) - margin
		if maxReceivable < recv {
			maxReceivable = recv
		}
	}
	return maxReceivable, nil
}

func hashHexString(hexString string) ([]byte, error) {
	hexBytes, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, err
	}
	hashed := sha256.Sum256([]byte(hexBytes))
	return hashed[:], nil
}
