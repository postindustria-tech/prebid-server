package openrtb_ext

import (
	"testing"

	"github.com/prebid/prebid-server/v4/util/jsonutil"
	"github.com/stretchr/testify/assert"
)

func TestExtRequestPrebidAdServerUnmarshalKeepsTypeErrorForValidate(t *testing.T) {
	var ext ExtRequest

	err := jsonutil.Unmarshal([]byte(`{"prebid":{"debug":true,"as":{"ap":true,"tmax_ms":"150"}}}`), &ext)

	assert.NoError(t, err)
	assert.True(t, ext.Prebid.Debug)
	assert.ErrorContains(t, ext.Prebid.AdServer.Validate(), "request.ext.prebid.as is invalid and was ignored")
}

func TestExtRequestPrebidAdServerTypeErrorSurvivesMarshal(t *testing.T) {
	var ext ExtRequest
	assert.NoError(t, jsonutil.Unmarshal([]byte(`{"prebid":{"as":{"ap":"yes"}}}`), &ext))

	b, err := jsonutil.Marshal(ext)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"prebid":{"as":{"ap":"yes"}}}`, string(b))

	var reparsed ExtRequest
	assert.NoError(t, jsonutil.Unmarshal(b, &reparsed))
	assert.Error(t, reparsed.Prebid.AdServer.Validate())
}

func TestExtRequestPrebidAdServerMarshalValid(t *testing.T) {
	b, err := jsonutil.Marshal(&ExtRequestPrebidAdServer{AllowPriority: true, AllowPriorityBidders: []string{"a"}})

	assert.NoError(t, err)
	assert.JSONEq(t, `{"ap":true,"apb":["a"]}`, string(b))
}

func TestExtRequestPrebidAdServerValidate(t *testing.T) {
	testCases := []struct {
		name          string
		as            string
		expectedError string
	}{
		{name: "all-fields-valid", as: `{"ap":true,"apb":["a"],"mode":"pre-auction","tmax_ms":150,"floor":{"enabled":true,"source":"cpm"},"short_circuit":{"enabled":true,"max_priority":0}}`},
		{name: "empty", as: `{}`},
		{name: "unknown-mode", as: `{"mode":"serial"}`, expectedError: `request.ext.prebid.as.mode must be "parallel" or "pre-auction"`},
		{name: "unknown-floor-source", as: `{"floor":{"source":"price"}}`, expectedError: `request.ext.prebid.as.floor.source must be "priority_adjusted_cpm" or "cpm"`},
		{name: "ranges-are-left-to-the-code-that-uses-them", as: `{"ap":true,"tmax_ms":0,"short_circuit":{"max_priority":-1}}`},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var as ExtRequestPrebidAdServer
			assert.NoError(t, jsonutil.Unmarshal([]byte(tc.as), &as))

			err := as.Validate()

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
	assert.NoError(t, (*ExtRequestPrebidAdServer)(nil).Validate())
}
