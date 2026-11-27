package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleBid(t *testing.T) {
	body := `{"id":"req","imp":[
		{"id":"open","banner":{"format":[{"w":300,"h":250}]},"ext":{"bidder":{"placementId":1}}},
		{"id":"deal","banner":{"format":[{"w":300,"h":250}]},"ext":{"bidder":{"placementId":2}}},
		{"id":"none","ext":{"bidder":{"placementId":99}}}]}`
	rec := httptest.NewRecorder()
	handleBid(rec, httptest.NewRequest(http.MethodPost, "/bid", strings.NewReader(body)))

	var resp struct {
		SeatBid []struct {
			Bid []struct {
				ImpID  string  `json:"impid"`
				Price  float64 `json:"price"`
				DealID string  `json:"dealid"`
				Ext    struct {
					Prebid struct {
						Deal map[string]any `json:"deal"`
					} `json:"prebid"`
				} `json:"ext"`
			} `json:"bid"`
		} `json:"seatbid"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("status %d, body %q: %v", rec.Code, rec.Body.String(), err)
	}
	bids := resp.SeatBid[0].Bid
	if len(bids) != 2 {
		t.Fatalf("got %d bids, want 2", len(bids))
	}
	if open := bids[0]; open.ImpID != "open" || open.DealID != "" || open.Ext.Prebid.Deal != nil || open.Price < 0.5 || open.Price > 2.5 {
		t.Errorf("open market bid = %+v", open)
	}
	if deal := bids[1]; deal.ImpID != "deal" || deal.DealID != "dealsim-sponsorship" || deal.Ext.Prebid.Deal["type"] != 1.0 || deal.Ext.Prebid.Deal["cpm"] != 20.0 {
		t.Errorf("sponsorship bid = %+v", deal)
	}
}

func TestHandleBidNoBid(t *testing.T) {
	rec := httptest.NewRecorder()
	handleBid(rec, httptest.NewRequest(http.MethodPost, "/bid", strings.NewReader(`{"id":"req","imp":[{"id":"1"}]}`)))
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
