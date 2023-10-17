package lspclient

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btclog"
	"github.com/lightningnetwork/lnd/lnrpc"
	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
	"google.golang.org/protobuf/proto"
)

const (
	// loop limit for lspclient.QueryRoutes
	lspCallMax = 10
)

func createNodePair(from string, to string) (*lspdrpc.NodePair, error) {
	var pair lspdrpc.NodePair
	var err error
	pair.From, err = hex.DecodeString(from)
	if err != nil {
		return nil, errFormat(errQueryRoutesRoute, "DecodeString(from)", err)
	}
	pair.To, err = hex.DecodeString(to)
	if err != nil {
		return nil, errFormat(errQueryRoutesRoute, "DecodeString(to)", err)
	}
	return &pair, nil
}

func (lc *LspClient) queryRoutePayment(chanInfo *lspdrpc.ChannelInformationReply, invoice string, feeLimitSat int32, amtSat int64) (paymentHash string, status lnrpc.Payment_PaymentStatus, failureReason lnrpc.PaymentFailureReason, err error) {
	log.Trace("queryRoutePayment")
	var errPairs []*lspdrpc.NodePair
	status = lnrpc.Payment_FAILED
	failureReason = lnrpc.PaymentFailureReason_FAILURE_REASON_ERROR

	payReq, err := lc.LndApi.DecodePayReq(invoice)
	if err != nil {
		err = errFormat(errQueryRoutes, "decodePayReq", err)
		return
	}
	paymentHash = payReq.PaymentHash
	if payReq.Timestamp+payReq.Expiry < time.Now().Unix() {
		err = errQueryRoutesExpired
		return
	}
	if payReq.NumMsat == 0 {
		log.Debugf("queryRoutePayment: use user input amount")
		payReq.NumMsat = amtSat * 1000
	}
	info, err := lc.LndApi.GetInfo()
	if err == nil {
		log.Debugf("current block height: %v", info.BlockHeight)
	}

	var reportLevel lspdrpc.ReportRequest_ReportLevel
	for lp := 0; lp < lspCallMax; lp++ {
		var routes *lnrpc.Route
		var errPair *lspdrpc.NodePair
		routes, err = lc.queryRoutesRoute(chanInfo, invoice, feeLimitSat, payReq, errPairs, amtSat)
		if err != nil {
			// fail LSP route creation
			log.Debugf("queryRoutePayment(route): err: %v", err)
			failureReason = lnrpc.PaymentFailureReason_FAILURE_REASON_NO_ROUTE
			reportLevel = lspdrpc.ReportRequest_REPORTLEVEL_NOTIFY
			err = nil
			break
		}
		errPair, failureReason, err = lc.queryRoutePay(invoice, routes)
		if err != nil {
			// payment API error(not payment error)
			log.Errorf("queryRoutePayment(pay): err: %v", err)
			reportLevel = lspdrpc.ReportRequest_REPORTLEVEL_NORMAL
			break
		}
		if failureReason == lnrpc.PaymentFailureReason_FAILURE_REASON_NONE {
			// payment success
			log.Info("queryRoutePayment(pay) Done.")
			status = lnrpc.Payment_SUCCEEDED
			break
		}
		log.Debugf("queryRoutePayment(pay): failureReason: %v", failureReason)
		if errPair == nil {
			// payment fail and no failure channel
			reportLevel = lspdrpc.ReportRequest_REPORTLEVEL_NORMAL
			break
		}
		log.Tracef("  add ignore node: %x->%x", errPair.From, errPair.To)
		errPairs = append(errPairs, errPair)
		reportLevel = lspdrpc.ReportRequest_REPORTLEVEL_NOTIFY
	}
	if status != lnrpc.Payment_SUCCEEDED {
		// error report
		var destString string
		hashedDestination, err2 := hashHexString(payReq.Destination)
		if err2 == nil {
			destString = hex.EncodeToString(hashedDestination[:])
		} else {
			destString = fmt.Sprintf("queryRoutePayment(hashHexString err): %v", err2)
			log.Errorf(destString)
		}
		message := fmt.Sprintf("FAIL: hashed_destination=%s, errPairs=%d, last_reason=%v",
			destString, len(errPairs), failureReason)
		lc.reportMessage(chanInfo, reportCategory, reportLevel, message)
	}
	return paymentHash, status, failureReason, err
}

// queryRoutesRoute request LSP to create payment route.
func (lc *LspClient) queryRoutesRoute(chanInfo *lspdrpc.ChannelInformationReply, invoice string, feeLimitSat int32, payReq *lnrpc.PayReq, errPairs []*lspdrpc.NodePair, amtSat int64) (*lnrpc.Route, error) {
	// 返信用privkey
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, errFormat(errQueryRoutesRoute, "NewPrivateKey", err)
	}

	lspPubkeyBytes, err := btcec.ParsePubKey(chanInfo.LspPubkey)
	if err != nil {
		return nil, errFormat(errQueryRoutesRoute, "ParsePubKey", err)
	}

	// query routes
	qrReq := &lspdrpc.QueryRoutesRequest{
		EncryptPubkey: priv.PubKey().SerializeCompressed(),
		Invoice:       invoice,
		IgnoredPairs:  errPairs,
		Amount:        amtSat,
	}
	data, _ := proto.Marshal(qrReq)

	// request
	encrypted, err := Encrypt(lspPubkeyBytes, data)
	if err != nil {
		return nil, errFormat(errQueryRoutesRoute, "Encrypt", err)
	}
	req := &lspdrpc.Encrypted{
		Data: encrypted,
	}
	res, err := lc.Client.QueryRoutes(lc.Ctx, req)
	if err != nil {
		return nil, errFormat(errQueryRoutesRoute, "QueryRoutes", err)
	}
	// reply
	decoded, err := Decrypt(priv, res.Data)
	if err != nil {
		return nil, errFormat(errQueryRoutesRoute, "Decrypt", err)
	}
	var rsp lnrpc.QueryRoutesResponse
	err = proto.Unmarshal(decoded, &rsp)
	if err != nil {
		return nil, errFormat(errQueryRoutesRoute, "Unmarshal", err)
	}
	if len(rsp.Routes) == 0 || rsp.Routes[0].Hops == nil {
		return nil, errFormat(errQueryRoutesRoute, "no routes", nil)
	}
	// log.Tracef("queryRoutesRoute: before routes=%d", len(rsp.Routes))
	// showRoutes(rsp.Routes)

	// use first route
	hubRoute := rsp.Routes[0]

	// add payment_addr
	if payReq.PaymentAddr != nil {
		hubRoute.Hops[len(hubRoute.Hops)-1].MppRecord = &lnrpc.MPPRecord{
			PaymentAddr:  payReq.PaymentAddr,
			TotalAmtMsat: payReq.NumMsat,
		}
	}

	// add route from our node to HUB node
	// 最初に見つかった送金可能な local balance のチャネルをルートの先頭に追加する
	log.Tracef("feeLimitSat=%d", feeLimitSat)
	channels, err := lc.LndApi.ListChannels()
	if err != nil {
		return nil, errFormat(errQueryRoutesRoute, "listChannels", err)
	}
	hubFeeMsat := hubHopFeeMsat(chanInfo, hubRoute.TotalAmtMsat)
	amountSat := (hubRoute.TotalAmtMsat + hubFeeMsat) / 1000
	feeSat := (hubRoute.TotalFeesMsat + hubFeeMsat) / 1000
	log.Tracef("  amountSat: %d", amountSat)
	log.Tracef("  feeSat   : %d", feeSat)
	var routeChanId uint64
	for _, b := range channels.Channels {
		// HUBに払うfee以上のlocal balanceを持つこと
		log.Tracef("  LocalBalance(%d): %d", b.ChanId, b.LocalBalance)
		if b.LocalBalance >= amountSat && feeSat < int64(feeLimitSat) {
			routeChanId = b.ChanId
			break
		}
	}
	if routeChanId == 0 {
		return nil, errFormat(errQueryRoutesRoute, "local balance not found", nil)
	}
	// hubRouteはHUBからのルートになっているため先頭にNC2->HUBのルートを追加する。
	// 'TotalXxx'が最初の update_add_htlc のパラメータで、Hopsはonionのデータになる。
	head := []*lnrpc.Hop{
		{
			ChanId:           routeChanId,
			AmtToForwardMsat: hubRoute.TotalAmtMsat,
			FeeMsat:          hubFeeMsat,
			Expiry:           hubRoute.TotalTimeLock,
			PubKey:           chanInfo.Pubkey,
		},
	}
	hubRoute.Hops = append(head, hubRoute.Hops...)
	// update total data
	hubRoute.TotalAmtMsat += hubFeeMsat
	hubRoute.TotalFeesMsat += hubFeeMsat
	hubRoute.TotalTimeLock += chanInfo.TimeLockDelta
	log.Tracef("queryRoutes: done")
	showRoutes([]*lnrpc.Route{hubRoute})
	return hubRoute, nil
}

func showRoutes(routes []*lnrpc.Route) {
	if log.Level() != btclog.LevelTrace {
		return
	}
	log.Tracef("routes len: %d", len(routes))
	for idx, v := range routes {
		log.Tracef("route[%d]", idx)
		log.Tracef("  total_time_lock: %d", v.TotalTimeLock)
		log.Tracef("  total_fees_msat: %d", v.TotalFeesMsat)
		log.Tracef("  total_amt_msat: %d", v.TotalAmtMsat)
		log.Tracef("  hops:")
		for hidx, hop := range v.Hops {
			log.Tracef("   hop[%d]", hidx)
			log.Tracef("    chan_id: %d", hop.ChanId)
			log.Tracef("    amt_to_forward_msat: %d", hop.AmtToForwardMsat)
			log.Tracef("    fee_msat: %d", hop.FeeMsat)
			log.Tracef("    expiry: %d", hop.Expiry)
			log.Tracef("    pub_key: %s", hop.PubKey)
			if hop.MppRecord != nil {
				log.Tracef("    mpp_record:")
				log.Tracef("      payment_addr: %x", hop.MppRecord.PaymentAddr)
				log.Tracef("      total_amt_msat: %d", hop.MppRecord.TotalAmtMsat)
			} else {
				log.Tracef("    mpp_record: nil")
			}
		}
	}
	log.Tracef("")
}

// queryRoutePay request route-payment.
func (lc *LspClient) queryRoutePay(invoice string, route *lnrpc.Route) (errPair *lspdrpc.NodePair, failureReason lnrpc.PaymentFailureReason, err error) {
	failureReason = lnrpc.PaymentFailureReason_FAILURE_REASON_ERROR
	payReq, _ := lc.LndApi.DecodePayReq(invoice)
	hash, _ := hex.DecodeString(payReq.PaymentHash)
	var payRes *lnrpc.SendResponse
	payRes, err = lc.LndApi.SendToRouteSync(hash, route)
	if err != nil {
		err = errFormat(errQueryRoutesPay, "sendToRouteSync", err)
		return
	}

	log.Tracef("PaymentHash: %x", payRes.PaymentHash)
	if len(payRes.PaymentError) == 0 {
		log.Debugf("queryRoutePay Success!: PaymentHash=%x", payRes.PaymentHash)
		if log.Level() == btclog.LevelTrace {
			for i, v := range payRes.PaymentRoute.Hops {
				log.Tracef("    [%d]: %d", i, v.ChanId)
				log.Tracef("       pubkey: %s", v.PubKey)
				log.Tracef("       amt_to_forward_msat: %d", v.AmtToForwardMsat)
				log.Tracef("       fee_msat: %d", v.FeeMsat)
			}
		}
		failureReason = lnrpc.PaymentFailureReason_FAILURE_REASON_NONE
		err = nil
		return
	}

	log.Debugf("  Error: %s", payRes.PaymentError)
	listRes, err := lc.LndApi.ListPayments()
	if err != nil {
		err = errFormat(errQueryRoutesPay, "listPayments", err)
		return
	}
	for _, v := range listRes.Payments {
		if v.PaymentHash != hex.EncodeToString(payRes.PaymentHash) {
			continue
		}
		failureReason = v.FailureReason
		log.Debugf("  status: %s", v.Status)
		log.Debugf("  failure_reason: %s", v.FailureReason.String())
		if v.FailureReason == lnrpc.PaymentFailureReason_FAILURE_REASON_NONE {
			// maybe already paied
			err = fmt.Errorf(payRes.PaymentError)
			return
		}
		if len(v.Htlcs) != 1 {
			err = fmt.Errorf(payRes.PaymentError)
			return
		}
		vv := v.Htlcs[0]
		if vv.Failure.FailureSourceIndex == 0 || vv.Failure.FailureSourceIndex >= uint32(len(vv.Route.Hops)) {
			// FailureSourceIndex
			//		0 ... sender node
			//		len(Hops) ... final node?
			err = fmt.Errorf("PaymentError=%s, FailureSourceIndex=%d", payRes.PaymentError, vv.Failure.FailureSourceIndex)
			return
		}
		toIdx := vv.Failure.FailureSourceIndex
		fromIdx := toIdx - 1
		errPair, err = createNodePair(vv.Route.Hops[fromIdx].PubKey, vv.Route.Hops[toIdx].PubKey)
		if err != nil {
			return
		}
		// log.Debugf(" vv.Failure: %v", vv.Failure)
		log.Debugf("    failure: %s", vv.Failure.Code.String())
		log.Debugf("     status: %s", vv.Status.String())
		if log.Level() == btclog.LevelTrace {
			log.Tracef("     srcidx: %d", vv.Failure.FailureSourceIndex)
			log.Tracef("       from: %s", vv.Route.Hops[fromIdx].PubKey)
			log.Tracef("         to: %s", vv.Route.Hops[toIdx].PubKey)
			for iii, vvv := range vv.Route.Hops {
				log.Tracef("      [%d] chan_id: %d", iii, vvv.ChanId)
				log.Tracef("        pubkey: %s", vvv.PubKey)
				log.Tracef("        amt_to_forward_msat: %d", vvv.AmtToForwardMsat)
			}
		}
		break
	}
	return
}
