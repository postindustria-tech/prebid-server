package openrtb_ext

import (
	"encoding/json"
	"testing"

	"github.com/prebid/prebid-server/v4/util/jsonutil"
	"github.com/prebid/prebid-server/v4/util/ptrutil"
	"github.com/stretchr/testify/assert"
)

func TestParseDeal(t *testing.T) {
	testCases := []struct {
		name          string
		dealID        string
		raw           json.RawMessage
		expectedDeal  *ExtBidPrebidDeal
		expectedError string
	}{
		{
			name:   "all-fields",
			dealID: "sponsor-123",
			raw:    json.RawMessage(`{"type":1,"cpm":5,"priority_adjusted_cpm":8.4,"on_schedule":0.8,"priority":4,"source":"adapter"}`),
			expectedDeal: &ExtBidPrebidDeal{
				Type:                DealTypeSponsorship,
				CPM:                 ptrutil.ToPtr(5.0),
				PriorityAdjustedCPM: ptrutil.ToPtr(8.4),
				OnSchedule:          ptrutil.ToPtr(0.8),
				Priority:            ptrutil.ToPtr(4),
			},
		},
		{
			name:         "only-type",
			dealID:       "house-1",
			raw:          json.RawMessage(`{"type":4}`),
			expectedDeal: &ExtBidPrebidDeal{Type: DealTypeHouse},
		},
		{
			name:         "badly-typed-optional-fields-are-left-out",
			dealID:       "deal-1",
			raw:          json.RawMessage(`{"type":3,"cpm":-1,"priority_adjusted_cpm":"8.4","on_schedule":null,"priority":4.5}`),
			expectedDeal: &ExtBidPrebidDeal{Type: DealTypeNonReservation},
		},
		{
			name:         "number-out-of-float64-range-drops-only-its-field",
			dealID:       "deal-1",
			raw:          json.RawMessage(`{"type":3,"cpm":1e999,"priority":2}`),
			expectedDeal: &ExtBidPrebidDeal{Type: DealTypeNonReservation, Priority: ptrutil.ToPtr(2)},
		},
		{
			name:          "reservation-with-cpm-out-of-range",
			dealID:        "deal-1",
			raw:           json.RawMessage(`{"type":2,"cpm":1e999}`),
			expectedError: "deal cpm is required for sponsorship and reservation deals",
		},
		{
			name:         "negative-priority-is-kept",
			dealID:       "deal-1",
			raw:          json.RawMessage(`{"type":3,"priority":-1}`),
			expectedDeal: &ExtBidPrebidDeal{Type: DealTypeNonReservation, Priority: ptrutil.ToPtr(-1)},
		},
		{
			name:         "priority-above-12",
			dealID:       "house-1",
			raw:          json.RawMessage(`{"type":4,"priority":16}`),
			expectedDeal: &ExtBidPrebidDeal{Type: DealTypeHouse, Priority: ptrutil.ToPtr(16)},
		},
		{
			name:          "no-dealid",
			raw:           json.RawMessage(`{"type":1,"cpm":5}`),
			expectedError: "deal metadata requires a non-empty bid.dealid",
		},
		{
			name:          "not-an-object",
			dealID:        "deal-1",
			raw:           json.RawMessage(`[1]`),
			expectedError: "deal metadata must be a JSON object",
		},
		{
			name:          "null",
			dealID:        "deal-1",
			raw:           json.RawMessage(`null`),
			expectedError: "deal metadata must be a JSON object",
		},
		{
			name:          "type-missing",
			dealID:        "deal-1",
			raw:           json.RawMessage(`{"cpm":5}`),
			expectedError: "deal type must be an integer from 0 to 4",
		},
		{
			name:          "type-out-of-range",
			dealID:        "deal-1",
			raw:           json.RawMessage(`{"type":5}`),
			expectedError: "deal type must be an integer from 0 to 4",
		},
		{
			name:          "type-not-integer",
			dealID:        "deal-1",
			raw:           json.RawMessage(`{"type":1.5,"cpm":5}`),
			expectedError: "deal type must be an integer from 0 to 4",
		},
		{
			name:          "reservation-without-cpm",
			dealID:        "deal-1",
			raw:           json.RawMessage(`{"type":2,"priority":6}`),
			expectedError: "deal cpm is required for sponsorship and reservation deals",
		},
		{
			name:          "sponsorship-with-invalid-cpm",
			dealID:        "deal-1",
			raw:           json.RawMessage(`{"type":1,"cpm":"5"}`),
			expectedError: "deal cpm is required for sponsorship and reservation deals",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deal, err := ParseDeal(tc.dealID, tc.raw)

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				assert.Nil(t, deal)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedDeal, deal)
			}
		})
	}
}

func TestExtBidPrebidDealUnmarshalNeverFails(t *testing.T) {
	testCases := []struct {
		name         string
		ext          string
		expectedDeal *ExtBidPrebidDeal
	}{
		{
			name:         "string",
			ext:          `{"deal":"x"}`,
			expectedDeal: &ExtBidPrebidDeal{},
		},
		{
			name:         "badly-typed-fields",
			ext:          `{"deal":{"type":"x","cpm":"5","priority":3,"source":"adserver"}}`,
			expectedDeal: &ExtBidPrebidDeal{Priority: ptrutil.ToPtr(3), Source: "adserver"},
		},
		{
			name: "null",
			ext:  `{"deal":null}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			type container struct {
				Deal *ExtBidPrebidDeal `json:"deal"`
			}
			var strict, lenient container

			assert.NoError(t, jsonutil.UnmarshalValid([]byte(tc.ext), &strict))
			assert.NoError(t, jsonutil.Unmarshal([]byte(tc.ext), &lenient))
			assert.Equal(t, tc.expectedDeal, strict.Deal)
			assert.Equal(t, tc.expectedDeal, lenient.Deal)
		})
	}
}

func TestExtBidUnmarshalWithBadlyTypedDeal(t *testing.T) {
	for _, ext := range []string{
		`{"prebid":{"type":"banner","deal":"x"}}`,
		`{"prebid":{"type":"banner","deal":{"type":"x","cpm":"5"}}}`,
	} {
		var bidExt ExtBid
		assert.NoError(t, jsonutil.UnmarshalValid([]byte(ext), &bidExt), ext)
		assert.Equal(t, BidTypeBanner, bidExt.Prebid.Type, ext)
	}
}
