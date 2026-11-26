package exchange

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/prebid/openrtb/v20/openrtb2"
	"github.com/prebid/prebid-server/v4/errortypes"
	"github.com/prebid/prebid-server/v4/exchange/entities"
	"github.com/prebid/prebid-server/v4/openrtb_ext"
	"github.com/prebid/prebid-server/v4/util/jsonutil"
)

// trustedDeal returns the bid's ext.prebid.deal when the bid seat is the called bidder, that bidder
// is in apb, and the object is valid. Bids under alternate bidder codes are never trusted: with
// alternatebiddercodes an adapter can return any seat, including a trusted one. Prebid.js also
// looks up trust by the seat, but it cannot tell which bidder returned it, so this check is stricter.
// convert moves the deal amounts into the currency of the bid price. An error is the reason PBS
// removed the deal. Without ap the deal is removed silently: the request did not ask for deals,
// so a reason on every such bid would only be noise.
//
// The raw deal is deleted from bid.Ext, because the cache stores bid.Ext as the adapter sent it,
// and an untrusted or unconverted deal must not reach it.
func trustedDeal(adServer *openrtb_ext.ExtRequestPrebidAdServer, bidder, seat string, bid *openrtb2.Bid, convert func(float64) (float64, error)) (*openrtb_ext.ExtBidPrebidDeal, error) {
	ext, raw := takeRawDeal(bid.Ext)
	if raw == nil {
		return nil, nil
	}
	bid.Ext = ext
	_, dataType, _, err := jsonparser.Get(raw)
	if dataType == jsonparser.Null || adServer == nil || !adServer.AllowPriority {
		return nil, nil
	}

	var deal *openrtb_ext.ExtBidPrebidDeal
	switch {
	case !adServer.Trusts(bidder):
		err = fmt.Errorf("bidder %s is not in request.ext.prebid.as.apb", bidder)
	case !strings.EqualFold(bidder, seat):
		err = fmt.Errorf("seat %s is an alternate bidder code of %s", seat, bidder)
	case err != nil || dataType != jsonparser.Object:
		err = errors.New("deal metadata must be a JSON object")
	default:
		deal, err = openrtb_ext.ParseDeal(bid.DealID, raw)
	}
	if err != nil {
		return nil, err
	}

	for _, amount := range []*float64{deal.CPM, deal.PriorityAdjustedCPM} {
		if amount == nil {
			continue
		}
		converted, err := convert(*amount)
		if err != nil {
			return nil, fmt.Errorf("deal amount cannot be converted: %v", err)
		}
		if math.IsInf(converted, 0) || math.IsNaN(converted) {
			return nil, errors.New("deal amount is out of range after currency conversion")
		}
		*amount = converted
	}
	deal.Source = openrtb_ext.DealSourceAdServer
	return deal, nil
}

// takeRawDeal returns ext without ext.prebid.deal, and the deal it removed. It decodes the JSON
// instead of editing the bytes, so the trust check and the delete see the same deal: with duplicate
// keys the last one wins, as in the response and in JSON.parse on the client. The returned ext has
// no duplicate prebid or deal keys left. An ext that does not decode is returned unchanged with no
// deal; makeBidExtJSON rejects such a bid later.
//
// Almost no bid has a deal, so ext is decoded only when it contains the key "deal" in quotes or a
// backslash. The backslash catches a key written with escapes, such as "\u0064eal", which still
// decodes to deal.
func takeRawDeal(ext json.RawMessage) (json.RawMessage, json.RawMessage) {
	if !bytes.Contains(ext, []byte(`"deal"`)) && bytes.IndexByte(ext, '\\') < 0 {
		return ext, nil
	}
	fields, err := decodeObject(ext)
	if err != nil {
		return ext, nil
	}
	prebid, err := decodeObject(fields["prebid"])
	if err != nil {
		return ext, nil
	}
	deal, ok := prebid["deal"]
	if !ok {
		return ext, nil
	}
	delete(prebid, "deal")
	if fields["prebid"], err = jsonutil.Marshal(prebid); err != nil {
		return ext, nil
	}
	rest, err := jsonutil.Marshal(fields)
	if err != nil {
		return ext, nil
	}
	return rest, deal
}

// decodeObject decodes a JSON object into its raw fields. jsoniter decodes a JSON null into a nil
// RawMessage, and the encoder that main.go registers writes nothing for nil, which would leave
// {"meta":} in the ext. So nulls are kept as the text null.
func decodeObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := jsonutil.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	for key, value := range fields {
		if value == nil {
			fields[key] = json.RawMessage("null")
		}
	}
	return fields, nil
}

func removedDealWarning(seat, bidID, reason string) error {
	return &errortypes.DebugWarning{
		WarningCode: errortypes.InvalidBidDealWarningCode,
		Message:     fmt.Sprintf("seat %s, bid %s: ext.prebid.deal was removed: %s", seat, bidID, reason),
	}
}

// strippedDeals lists the deals PBS removed, for ext.debug.as. It sees only bids that are still
// in the auction, so a bid that was rejected later (for example by floors) is not listed.
func strippedDeals(adapterBids map[openrtb_ext.BidderName]*entities.PbsOrtbSeatBid) []openrtb_ext.ExtResponseDebugAdServerStrippedDeal {
	var stripped []openrtb_ext.ExtResponseDebugAdServerStrippedDeal
	for seat, seatBid := range adapterBids {
		if seatBid == nil {
			continue
		}
		for _, bid := range seatBid.Bids {
			if bid.DealRemovedReason != "" {
				stripped = append(stripped, openrtb_ext.ExtResponseDebugAdServerStrippedDeal{
					Seat:   seat.String(),
					ImpID:  bid.Bid.ImpID,
					BidID:  bid.Bid.ID,
					Reason: bid.DealRemovedReason,
				})
			}
		}
	}
	slices.SortFunc(stripped, func(a, b openrtb_ext.ExtResponseDebugAdServerStrippedDeal) int {
		return cmp.Or(cmp.Compare(a.Seat, b.Seat), cmp.Compare(a.ImpID, b.ImpID), cmp.Compare(a.BidID, b.BidID))
	})
	return stripped
}
