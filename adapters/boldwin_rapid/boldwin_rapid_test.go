package boldwin_rapid

import (
	"testing"

	"github.com/prebid/prebid-server/v3/adapters/adapterstest"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/openrtb_ext"
)

func TestJsonSamples(t *testing.T) {
	bidder, buildErr := Builder(
		openrtb_ext.BidderBoldwinRapid,
		config.Adapter{
			Endpoint: "https://rtb.beardfleet.com/auction/bid?pid={{.PublisherID}}&tid={{.PlacementID}}",
		},
		config.Server{
			ExternalUrl: "http://hosturl.com",
			GvlID:       1, DataCenter: "2",
		},
	)

	if buildErr != nil {
		t.Fatalf("Builder returned unexpected error %v", buildErr)
	}

	adapterstest.RunJSONBidderTest(t, "boldwintest", bidder)
}
