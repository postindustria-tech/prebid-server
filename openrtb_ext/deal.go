package openrtb_ext

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/buger/jsonparser"
)

type DealType int

const (
	DealTypeUnknown        DealType = 0
	DealTypeSponsorship    DealType = 1
	DealTypeReservation    DealType = 2
	DealTypeNonReservation DealType = 3
	DealTypeHouse          DealType = 4
)

// DealSourceAdServer marks deal metadata that PBS checked and passed on from a trusted seat.
const DealSourceAdServer = "adserver"

// ExtBidPrebidDeal defines the contract for bidresponse.seatbid.bid[i].ext.prebid.deal, the deal
// metadata an ad server adapter returns on a bid.
type ExtBidPrebidDeal struct {
	Type                DealType `json:"type"`
	CPM                 *float64 `json:"cpm,omitempty"`
	PriorityAdjustedCPM *float64 `json:"priority_adjusted_cpm,omitempty"`
	OnSchedule          *float64 `json:"on_schedule,omitempty"`
	Priority            *int     `json:"priority,omitempty"`
	Source              string   `json:"source,omitempty"`
}

// UnmarshalJSON never fails. Many readers decode the whole bid.ext, and a badly typed deal from
// an adapter must not break them. Bad fields are left out, as ParseDeal does.
func (deal *ExtBidPrebidDeal) UnmarshalJSON(b []byte) error {
	*deal = ExtBidPrebidDeal{}
	fields, err := readDealFields(b)
	if err != nil {
		return nil
	}
	if dealType, ok := parseInteger(fields["type"]); ok && validDealType(dealType) {
		deal.Type = DealType(dealType)
	}
	deal.readOptionalFields(fields)
	if source := fields["source"]; source.dataType == jsonparser.String {
		deal.Source, _ = jsonparser.ParseString(source.value)
	}
	return nil
}

// ParseDeal validates bid.ext.prebid.deal and returns only its known, correctly typed fields,
// following getDealMeta in Prebid.js. An error means the whole object must be dropped: the bid
// has no dealid, the object is not a JSON object, type is missing or outside 0..4, or a
// sponsorship or reservation has no valid cpm. Other badly typed fields are left out.
// Source is never copied, because PBS sets it itself on deals it honours.
func ParseDeal(dealID string, raw []byte) (*ExtBidPrebidDeal, error) {
	if dealID == "" {
		return nil, errors.New("deal metadata requires a non-empty bid.dealid")
	}

	fields, err := readDealFields(raw)
	if err != nil {
		return nil, errors.New("deal metadata must be a JSON object")
	}

	dealType, ok := parseInteger(fields["type"])
	if !ok || !validDealType(dealType) {
		return nil, fmt.Errorf("deal type must be an integer from %d to %d", DealTypeUnknown, DealTypeHouse)
	}

	deal := &ExtBidPrebidDeal{Type: DealType(dealType)}
	deal.readOptionalFields(fields)
	if deal.CPM == nil && (deal.Type == DealTypeSponsorship || deal.Type == DealTypeReservation) {
		return nil, errors.New("deal cpm is required for sponsorship and reservation deals")
	}
	return deal, nil
}

func (deal *ExtBidPrebidDeal) readOptionalFields(fields map[string]dealField) {
	if cpm, ok := parseNonNegativeNumber(fields["cpm"]); ok {
		deal.CPM = &cpm
	}
	if priorityAdjustedCPM, ok := parseNonNegativeNumber(fields["priority_adjusted_cpm"]); ok {
		deal.PriorityAdjustedCPM = &priorityAdjustedCPM
	}
	if onSchedule, ok := parseNonNegativeNumber(fields["on_schedule"]); ok {
		deal.OnSchedule = &onSchedule
	}
	if priority, ok := parseInteger(fields["priority"]); ok {
		deal.Priority = &priority
	}
}

type dealField struct {
	value    []byte
	dataType jsonparser.ValueType
}

// readDealFields keeps each value as raw bytes with its JSON type, so a number that does not fit
// a float64 drops only its own field.
func readDealFields(raw []byte) (map[string]dealField, error) {
	fields := make(map[string]dealField)
	err := jsonparser.ObjectEach(raw, func(key, value []byte, dataType jsonparser.ValueType, _ int) error {
		fields[string(key)] = dealField{value: value, dataType: dataType}
		return nil
	})
	return fields, err
}

func validDealType(dealType int) bool {
	return dealType >= int(DealTypeUnknown) && dealType <= int(DealTypeHouse)
}

func parseNumber(field dealField) (float64, bool) {
	if field.dataType != jsonparser.Number {
		return 0, false
	}
	value, err := strconv.ParseFloat(string(field.value), 64)
	return value, err == nil && !math.IsInf(value, 0) && !math.IsNaN(value)
}

func parseNonNegativeNumber(field dealField) (float64, bool) {
	value, ok := parseNumber(field)
	return value, ok && value >= 0
}

// parseInteger caps values at the int32 range so the conversion to int is exact on every
// platform. This is stricter than Prebid.js, which accepts any integer.
func parseInteger(field dealField) (int, bool) {
	value, ok := parseNumber(field)
	if !ok || value != math.Trunc(value) || value < math.MinInt32 || value > math.MaxInt32 {
		return 0, false
	}
	return int(value), true
}
