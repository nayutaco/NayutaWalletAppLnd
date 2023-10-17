package lspclient

import (
	"strconv"
	"strings"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	lspdrpc "github.com/nayutaco/NayutaHub2LspdProto"
)

func (lc *LspClient) selfRebalance(chanInfo *lspdrpc.ChannelInformationReply) error {
	log.Trace("selfRebalance: start")
	listRes, err := lc.LndApi.ListChannels()
	if err != nil {
		log.Errorf("selfRebalance: ListChannels err: %v", err)
		return errFormat(errSelfRebalance, "ListChannels", err)
	}
	if len(listRes.Channels) <= 1 {
		log.Tracef("selfRebalance: not need: channels=%d", len(listRes.Channels))
		return nil
	}

	waitChan := make(chan struct{})
	doneChan := make(chan struct{})
	go lc.selfRebalanceTransfer(chanInfo, waitChan, listRes)
	go lc.selfRebalanceClose(waitChan, doneChan)
	<-doneChan
	log.Trace("selfRebalance: done")

	return nil
}

func (lc *LspClient) selfRebalanceTransfer(chanInfo *lspdrpc.ChannelInformationReply, waitChan chan struct{}, listRes *lnrpc.ListChannelsResponse) {
	defer func() {
		waitChan <- struct{}{}
	}()

	log.Info("selfRebalanceTransfer: start")

	// maxChan.LocalConstraints.ChanReserveSat はゼロなので
	// 集約しきれないamountを考慮しないようにして単純化する
	var totalLocal int64
	var maxCapacity int64
	var maxCapacityIndex int
	for i, v := range listRes.Channels {
		if v.LocalConstraints.ChanReserveSat > 0 {
			// local reserve_sat MUST zero in this system.
			log.Error("selfRebalanceTransfer: local reserve_sat not ZERO")
			return
		}
		totalLocal += v.LocalBalance
		if v.Capacity > maxCapacity {
			maxCapacity = v.Capacity
			maxCapacityIndex = i
		}
		log.Tracef("  chan[%d]: %d", i, v.ChanId)
		log.Tracef("    capacity      : %d", v.Capacity)
		log.Tracef("    local_balance : %d", v.LocalBalance)
	}
	maxChan := listRes.Channels[maxCapacityIndex]
	cap := maxChan.LocalBalance + maxChan.RemoteBalance - int64(maxChan.RemoteConstraints.ChanReserveSat)
	if cap >= totalLocal {
		log.Tracef("Can aggregate: max cap[%d]=%v, total=%v", maxCapacityIndex, maxCapacity, totalLocal)
	} else {
		log.Warnf("selfRebalanceTransfer: Cannot aggregate: max cap[%v]=%v, total=%v", maxCapacityIndex, maxCapacity, totalLocal)
		return
	}

	// rebalance
	for i, v := range listRes.Channels {
		log.Infof("rebalance chan[%d]: %d, balance=%d", i, v.ChanId, v.LocalBalance)
		if i == maxCapacityIndex {
			log.Trace("  skip!: aggregate channel")
			continue
		}
		if v.LocalBalance == 0 {
			log.Trace("  skip!: local balance == 0")
			continue
		}

		resAddInvoice, err := lc.LndApi.AddInvoiceSimple(v.LocalBalance, "rebalance myself")
		if err != nil {
			log.Errorf("FAIL: AddInvoice: %v", err)
			continue
		}

		info, err := lc.LndApi.GetInfo()
		if err != nil {
			log.Error("getinfo: %w", err)
			return
		}
		log.Infof("curreht height: %d", info.BlockHeight)
		var payRoute lnrpc.Route
		payRoute.TotalFeesMsat = 0
		payRoute.TotalAmtMsat = v.LocalBalance * 1000
		payRoute.Hops = []*lnrpc.Hop{
			{
				ChanId:           v.ChanId,
				AmtToForwardMsat: payRoute.TotalAmtMsat,
				FeeMsat:          0,
				Expiry:           info.BlockHeight + lndConfBitcoinTimeLockDelay,
				PubKey:           chanInfo.Pubkey,
				TlvPayload:       true,
			},
			{
				ChanId:           listRes.Channels[maxCapacityIndex].RemoteChanId,
				AmtToForwardMsat: payRoute.TotalAmtMsat,
				FeeMsat:          0,
				Expiry:           info.BlockHeight + lndConfBitcoinTimeLockDelay,
				PubKey:           info.IdentityPubkey,
				TlvPayload:       true,
				MppRecord: &lnrpc.MPPRecord{
					PaymentAddr:  resAddInvoice.PaymentAddr,
					TotalAmtMsat: payRoute.TotalAmtMsat,
				},
			},
		}
		payRoute.TotalTimeLock = payRoute.Hops[0].Expiry + chanInfo.TimeLockDelta
		payRes, err := lc.LndApi.SendToRouteSync(resAddInvoice.RHash, &payRoute)
		if err != nil {
			log.Errorf("FAIL: SendToRouteSync: %v", err)
			continue
		}
		if len(payRes.PaymentError) == 0 {
			log.Infof("selfRebalanceTransfer: Payment Success!: Preimage=%x", payRes.PaymentPreimage)
			// for i, v := range payRes.PaymentRoute.Hops {
			// 	log.Tracef("    [%d]: %d", i, v.ChanId)
			// 	log.Tracef("       pubkey: %s", v.PubKey)
			// 	log.Tracef("       amt_to_forward_msat: %d", v.AmtToForwardMsat)
			// 	log.Tracef("       fee_msat: %d", v.FeeMsat)
			// }
		} else {
			log.Errorf("  Payment Error: %s", payRes.PaymentError)
		}
	}

	log.Info("selfRebalanceTransfer: done")
}

func (lc *LspClient) selfRebalanceClose(waitChan chan struct{}, doneChan chan struct{}) {
	var err error

	defer func() {
		doneChan <- struct{}{}
	}()

	// SendToRouteSync return after receiving 'update_fulfill_htlc' and 'commitment_signed'.
	// So wait a bit until both 'revoke_and_ack' is done.
	<-waitChan
	log.Info("selfRebalanceClose: start")
	time.Sleep(time.Second * 3)

	// close local zero-balance channels after rebalance
	listRes, err := lc.LndApi.ListChannels()
	if err != nil {
		log.Errorf("selfRebalanceClose: ListChannels err: %v", err)
		return
	}
	if len(listRes.Channels) <= 1 {
		log.Tracef("selfRebalanceClose: not need: channels=%d", len(listRes.Channels))
		return
	}
	for _, v := range listRes.Channels {
		log.Tracef("local balance: %v", v.LocalBalance)
		if v.LocalBalance != 0 {
			continue
		}
		chanPnt := strings.Split(v.ChannelPoint, ":")
		outputIndex, _ := strconv.Atoi(chanPnt[1])
		retry := 0
		for retry < rebalanceRetryClose {
			// fee = 1 sats/vbyte
			_, err = lc.LndApi.CloseChannel(chanPnt[0], uint32(outputIndex), 1)
			if err == nil {
				break
			}
			retry++
			log.Tracef("CloseChannel(%s) retry[%d] err: %v", v.ChannelPoint, retry, err)
			time.Sleep(time.Second * 5)
		}
		if err != nil {
			log.Errorf("CloseChannel(%s) err: %v", v.ChannelPoint, err)
			continue
		}
		log.Infof("CloseChannel(%s) start", v.ChannelPoint)
	}
	log.Info("selfRebalanceClose: done")
}
