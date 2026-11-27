// Command dealsim is a fake OpenRTB bidder for the 002_ad_server_deals sample. Prebid Server calls
// it through simadapter, whose endpoint the sample config points here, so dealsim runs outside
// Prebid Server like a real bidder. The placementId of each impression picks the answer, so one page
// can show every case side by side.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
)

type bidRequest struct {
	ID  string `json:"id"`
	Imp []imp  `json:"imp"`
}

type imp struct {
	ID     string `json:"id"`
	Banner *struct {
		Format []struct {
			W int64 `json:"w"`
			H int64 `json:"h"`
		} `json:"format"`
	} `json:"banner"`
	Ext struct {
		Bidder struct {
			PlacementID int `json:"placementId"`
		} `json:"bidder"`
	} `json:"ext"`
}

type bid struct {
	ID     string          `json:"id"`
	ImpID  string          `json:"impid"`
	Price  float64         `json:"price"`
	AdM    string          `json:"adm"`
	CrID   string          `json:"crid"`
	W      int64           `json:"w,omitempty"`
	H      int64           `json:"h,omitempty"`
	MType  int             `json:"mtype"`
	DealID string          `json:"dealid,omitempty"`
	Ext    json.RawMessage `json:"ext,omitempty"`
}

type answer struct {
	name   string
	price  float64
	dealID string
	deal   string
}

// answers maps a placementId to the bid dealsim returns. Prices are in USD. Any other placementId
// gets no bid.
var answers = map[int]answer{
	1: {name: "open market"},
	2: {name: "sponsorship", price: 4, dealID: "dealsim-sponsorship", deal: `{"type":1,"cpm":20,"priority":1}`},
	3: {name: "reservation", price: 3, dealID: "dealsim-reservation", deal: `{"type":2,"cpm":10,"priority_adjusted_cpm":12.5,"on_schedule":0.8,"priority":4}`},
	4: {name: "non-reservation", price: 2, dealID: "dealsim-pmp", deal: `{"type":3,"priority":8}`},
	5: {name: "house", price: 0.01, dealID: "dealsim-house", deal: `{"type":4,"priority":16}`},
	// Prebid Server removes this deal: a sponsorship needs a cpm.
	6: {name: "invalid sponsorship", price: 4, dealID: "dealsim-invalid", deal: `{"type":1,"priority":1}`},
}

func makeBid(imp imp) (bid, bool) {
	answer, ok := answers[imp.Ext.Bidder.PlacementID]
	if !ok {
		return bid{}, false
	}
	price := answer.price
	if answer.dealID == "" {
		price = 0.5 + rand.Float64()*2
	}
	b := bid{
		ID:     "dealsim-" + imp.ID,
		ImpID:  imp.ID,
		Price:  price,
		CrID:   fmt.Sprintf("dealsim-%d", imp.Ext.Bidder.PlacementID),
		DealID: answer.dealID,
		MType:  1, // banner
	}
	if imp.Banner != nil && len(imp.Banner.Format) > 0 {
		b.W, b.H = imp.Banner.Format[0].W, imp.Banner.Format[0].H
	}
	b.AdM = fmt.Sprintf(`<body style="margin:0"><div style="box-sizing:border-box;width:%dpx;height:%dpx;display:flex;flex-direction:column;justify-content:center;align-items:center;gap:6px;background:linear-gradient(135deg,#3b5bdb,#7048e8);color:#fff;font:600 20px sans-serif;text-align:center"><div style="font-size:12px;letter-spacing:2px;opacity:.75">DEALSIM</div><div>%s</div><div style="font-size:28px">$%.2f</div></div></body>`, b.W, b.H, answer.name, price)
	if answer.deal != "" {
		b.Ext = json.RawMessage(`{"prebid":{"deal":` + answer.deal + `}}`)
	}
	return b, true
}

func handleBid(w http.ResponseWriter, r *http.Request) {
	var req bidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var bids []bid
	for _, imp := range req.Imp {
		if b, ok := makeBid(imp); ok {
			bids = append(bids, b)
		}
	}
	log.Printf("request %s: %d imps, %d bids", req.ID, len(req.Imp), len(bids))
	if len(bids) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":      req.ID,
		"cur":     "USD",
		"seatbid": []map[string]any{{"bid": bids}},
	})
}

func main() {
	addr := ":8080"
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}
	http.HandleFunc("POST /bid", handleBid)
	log.Printf("dealsim listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
