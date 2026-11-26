package openrtb_ext

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/prebid/prebid-server/v4/util/jsonutil"
	"github.com/prebid/prebid-server/v4/util/ptrutil"
)

const (
	AdServerModeParallel   = "parallel"
	AdServerModePreAuction = "pre-auction"

	AdServerFloorSourcePriorityAdjustedCPM = "priority_adjusted_cpm"
	AdServerFloorSourceCPM                 = "cpm"
)

// ExtRequestPrebidAdServer defines the contract for bidrequest.ext.prebid.as, the ad server
// adapter configuration. Nothing in it takes effect unless AllowPriority is true.
type ExtRequestPrebidAdServer struct {
	AllowPriority        bool                                  `json:"ap,omitempty"`
	AllowPriorityBidders []string                              `json:"apb,omitempty"`
	Mode                 string                                `json:"mode,omitempty"`
	TMaxMS               *int64                                `json:"tmax_ms,omitempty"`
	Floor                *ExtRequestPrebidAdServerFloor        `json:"floor,omitempty"`
	ShortCircuit         *ExtRequestPrebidAdServerShortCircuit `json:"short_circuit,omitempty"`
	PriorityOffsets      map[string]int                        `json:"priority_offsets,omitempty"`

	// parseErr keeps a type error from UnmarshalJSON so Validate can report it as a warning
	// instead of failing the whole request.ext. raw keeps the original bytes so the error
	// survives a marshal and a new parse before validation.
	parseErr error
	raw      json.RawMessage
}

type ExtRequestPrebidAdServerFloor struct {
	Enabled bool   `json:"enabled,omitempty"`
	Source  string `json:"source,omitempty"`
}

type ExtRequestPrebidAdServerShortCircuit struct {
	Enabled     bool `json:"enabled,omitempty"`
	MaxPriority *int `json:"max_priority,omitempty"`
}

// UnmarshalJSON never fails. A malformed object must be dropped with a warning and the
// auction must go on, so the error is stored and returned later by Validate.
func (as *ExtRequestPrebidAdServer) UnmarshalJSON(b []byte) error {
	// use a type-alias to avoid calling back into this UnmarshalJSON implementation
	type adServer ExtRequestPrebidAdServer
	var parsed adServer
	if err := jsonutil.Unmarshal(b, &parsed); err != nil {
		*as = ExtRequestPrebidAdServer{parseErr: err, raw: slices.Clone(b)}
		return nil
	}
	*as = ExtRequestPrebidAdServer(parsed)
	return nil
}

func (as *ExtRequestPrebidAdServer) MarshalJSON() ([]byte, error) {
	if as.parseErr != nil {
		return as.raw, nil
	}
	type adServer ExtRequestPrebidAdServer
	return jsonutil.Marshal((*adServer)(as))
}

// Validate returns an error when the object is malformed and must be removed from the request.
func (as *ExtRequestPrebidAdServer) Validate() error {
	if as == nil {
		return nil
	}
	if as.parseErr != nil {
		// The decoder error names Go types and fields, which mean nothing to the caller.
		return errors.New("request.ext.prebid.as is invalid and was ignored: a field has the wrong type")
	}
	if as.Mode != "" && as.Mode != AdServerModeParallel && as.Mode != AdServerModePreAuction {
		return fmt.Errorf(`request.ext.prebid.as.mode must be "%s" or "%s"`, AdServerModeParallel, AdServerModePreAuction)
	}
	if as.Floor != nil && as.Floor.Source != "" &&
		as.Floor.Source != AdServerFloorSourcePriorityAdjustedCPM && as.Floor.Source != AdServerFloorSourceCPM {
		return fmt.Errorf(`request.ext.prebid.as.floor.source must be "%s" or "%s"`, AdServerFloorSourcePriorityAdjustedCPM, AdServerFloorSourceCPM)
	}
	return nil
}

// Trusts reports whether the bidder code is in apb with ap on. Codes are compared without case,
// as PBS does for bidder names in bid adjustments.
func (as *ExtRequestPrebidAdServer) Trusts(bidderCode string) bool {
	if as == nil || !as.AllowPriority {
		return false
	}
	return slices.ContainsFunc(as.AllowPriorityBidders, func(bidder string) bool {
		return strings.EqualFold(bidder, bidderCode)
	})
}

func (as *ExtRequestPrebidAdServer) Clone() *ExtRequestPrebidAdServer {
	if as == nil {
		return nil
	}
	clone := *as
	clone.raw = slices.Clone(as.raw)
	clone.AllowPriorityBidders = slices.Clone(as.AllowPriorityBidders)
	clone.TMaxMS = ptrutil.Clone(as.TMaxMS)
	if as.Floor != nil {
		floor := *as.Floor
		clone.Floor = &floor
	}
	if as.ShortCircuit != nil {
		shortCircuit := *as.ShortCircuit
		shortCircuit.MaxPriority = ptrutil.Clone(as.ShortCircuit.MaxPriority)
		clone.ShortCircuit = &shortCircuit
	}
	clone.PriorityOffsets = maps.Clone(as.PriorityOffsets)
	return &clone
}
