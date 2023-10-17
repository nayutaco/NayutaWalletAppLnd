package lspclient

import (
	"context"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/walletrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type LndApiConnector interface {
	Disconnect()
	GetInfo() (*lnrpc.GetInfoResponse, error)
	ListChannels() (*lnrpc.ListChannelsResponse, error)
	ListPayments() (*lnrpc.ListPaymentsResponse, error)
	AddInvoice(amountSat int64, preimage []byte, memo string, routeHint *lnrpc.HopHint, paymentAddr []byte) (string, []byte, []byte, error)
	AddInvoiceOnlyCreate(amountSat int64, preimage []byte, memo string, routeHint *lnrpc.HopHint, paymentAddr []byte) (string, []byte, []byte, error)
	AddInvoiceSimple(amountMsat int64, memo string) (*lnrpc.AddInvoiceResponse, error)
	DecodePayReq(invoice string) (*lnrpc.PayReq, error)
	AddWatchScript(script []byte, blockHashStr string, blockHeight uint32) (string, error)
	PublishTransaction(rawTx []byte, label string) error
	QueryRoutes(nodePubkeyBytes []byte, amountMsat int64, feeMsat int64) (*lnrpc.QueryRoutesResponse, error)
	SendToRouteSync(paymentHash []byte, route *lnrpc.Route) (*lnrpc.SendResponse, error)
	CloseChannel(txidStr string, outputIndex uint32, satPerVbyte uint64) (lnrpc.Lightning_CloseChannelClient, error)
}

type LndApi struct {
	LndConn         *grpc.ClientConn
	LndContext      context.Context
	RpcClient       lnrpc.LightningClient
	WalletKitClient walletrpc.WalletKitClient
}

const (
	// DefaultInvoiceExpiry is the default invoice expiry
	DefaultInvoiceExpiry = 1 * time.Hour
)

func newLndApi(cert, lndAddr, macaroon string) (*LndApi, error) {
	var err error

	cp := x509.NewCertPool()
	if !cp.AppendCertsFromPEM([]byte(cert)) {
		log.Error("credentials: failed to append certificates")
		return nil, errFormat(errNewLspClient, "AppendCertsFromPEM", err)
	}
	creds := credentials.NewClientTLSFromCert(cp, "")
	lndConn, err := grpc.Dial(lndAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		log.Errorf("Failed to connect to LND gRPC: %v", err)
		return nil, errFormat(errNewLspClient, "WithTransportCredentials", err)
	}
	lndContext := metadata.AppendToOutgoingContext(context.Background(), "macaroon", macaroon)
	rpcClient := lnrpc.NewLightningClient(lndConn)
	walletKitClient := walletrpc.NewWalletKitClient(lndConn)
	return &LndApi{
		LndConn:         lndConn,
		LndContext:      lndContext,
		RpcClient:       rpcClient,
		WalletKitClient: walletKitClient,
	}, nil
}

func (l *LndApi) Disconnect() {
	l.LndConn.Close()
}

func (l *LndApi) GetInfo() (*lnrpc.GetInfoResponse, error) {
	return l.RpcClient.GetInfo(l.LndContext, &lnrpc.GetInfoRequest{})
}

func (l *LndApi) ListChannels() (*lnrpc.ListChannelsResponse, error) {
	req := &lnrpc.ListChannelsRequest{
		ActiveOnly: true,
	}
	return l.RpcClient.ListChannels(l.LndContext, req)
}

func (l *LndApi) ListPayments() (*lnrpc.ListPaymentsResponse, error) {
	req := &lnrpc.ListPaymentsRequest{
		IncludeIncomplete: true,
		Reversed:          true,
	}
	return l.RpcClient.ListPayments(l.LndContext, req)
}

// AddInvoice create MPP not supported invoice and register LND DB
// This is for on-the-fly channel invoice (HUB --> NayutaWallet)
func (l *LndApi) AddInvoice(amountSat int64, preimage []byte, memo string, routeHint *lnrpc.HopHint, paymentAddr []byte) (string, []byte, []byte, error) {
	return l.addInvoiceInternal(amountSat, preimage, memo, routeHint, paymentAddr, false)
}

// AddInvoiceOnlyCreate create MPP not supported invoice and not register LND DB
// This is for on-the-fly channel invoice (Payer --> HUB)
func (l *LndApi) AddInvoiceOnlyCreate(amountSat int64, preimage []byte, memo string, routeHint *lnrpc.HopHint, paymentAddr []byte) (string, []byte, []byte, error) {
	return l.addInvoiceInternal(amountSat, preimage, memo, routeHint, paymentAddr, true)
}

func (l *LndApi) addInvoiceInternal(amountSat int64, preimage []byte, memo string, routeHint *lnrpc.HopHint, paymentAddr []byte, onlyCreate bool) (string, []byte, []byte, error) {
	var hints []*lnrpc.RouteHint
	hints = append(hints, &lnrpc.RouteHint{
		HopHints: []*lnrpc.HopHint{routeHint},
	})
	// On-the-fly expects to receive HTLC in a lump sum.
	// Therefore, create an INVOICE without MPP feature.
	req := &lnrpc.Invoice{
		Memo:        memo,
		Expiry:      int64(DefaultInvoiceExpiry.Seconds()),
		RPreimage:   preimage,
		Value:       amountSat,
		RouteHints:  hints,
		PaymentAddr: paymentAddr,
		OnlyCreate:  onlyCreate,
		IsNompp:     true,
	}
	res, err := l.RpcClient.AddInvoice(l.LndContext, req)
	if err != nil {
		return "", nil, nil, err
	}
	return res.PaymentRequest, res.RHash, res.PaymentAddr, nil
}

func (l *LndApi) AddInvoiceSimple(amountMsat int64, memo string) (*lnrpc.AddInvoiceResponse, error) {
	return l.RpcClient.AddInvoice(l.LndContext, &lnrpc.Invoice{
		Memo:      memo,
		Expiry:    int64(DefaultInvoiceExpiry.Seconds()),
		ValueMsat: amountMsat,
	})
}

func (l *LndApi) DecodePayReq(invoice string) (*lnrpc.PayReq, error) {
	return l.RpcClient.DecodePayReq(l.LndContext, &lnrpc.PayReqString{
		PayReq: invoice,
	})
}

func (l *LndApi) AddWatchScript(script []byte, blockHashStr string, blockHeight uint32) (string, error) {
	res, err := l.WalletKitClient.ImportWitnessScript(l.LndContext, &walletrpc.ImportWitnessScriptRequest{
		Script:       script,
		BlockHashStr: blockHashStr,
		BlockHeight:  int32(blockHeight),
	})
	if err != nil {
		return "", err
	}
	return res.Address, nil
}

func (l *LndApi) PublishTransaction(rawTx []byte, label string) error {
	res, err := l.WalletKitClient.PublishTransaction(l.LndContext, &walletrpc.Transaction{
		TxHex: rawTx,
		Label: label,
	})
	if err != nil {
		return err
	}
	if len(res.PublishError) != 0 {
		return fmt.Errorf(res.PublishError)
	}
	return nil
}

func (l *LndApi) QueryRoutes(nodePubkeyBytes []byte, amountMsat int64, feeMsat int64) (*lnrpc.QueryRoutesResponse, error) {
	return l.RpcClient.QueryRoutes(l.LndContext, &lnrpc.QueryRoutesRequest{
		PubKey:  hex.EncodeToString(nodePubkeyBytes),
		AmtMsat: amountMsat,
		FeeLimit: &lnrpc.FeeLimit{
			Limit: &lnrpc.FeeLimit_FixedMsat{
				FixedMsat: feeMsat,
			},
		},
	})
}

func (l *LndApi) SendToRouteSync(paymentHash []byte, route *lnrpc.Route) (*lnrpc.SendResponse, error) {
	req := &lnrpc.SendToRouteRequest{
		PaymentHash: paymentHash,
		Route:       route,
	}
	return l.RpcClient.SendToRouteSync(l.LndContext, req)
}

func (l *LndApi) CloseChannel(txidStr string, outputIndex uint32, satPerVbyte uint64) (lnrpc.Lightning_CloseChannelClient, error) {
	return l.RpcClient.CloseChannel(l.LndContext, &lnrpc.CloseChannelRequest{
		ChannelPoint: &lnrpc.ChannelPoint{
			FundingTxid: &lnrpc.ChannelPoint_FundingTxidStr{
				FundingTxidStr: txidStr,
			},
			OutputIndex: outputIndex,
		},
		SatPerVbyte: satPerVbyte,
	})
}
