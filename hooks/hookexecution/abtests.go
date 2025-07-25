package hookexecution

import (
	"math/rand"
	"slices"
	"sync"

	"github.com/golang/glog"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/prebid/prebid-server/v3/hooks/hookanalytics"
	"github.com/tidwall/gjson"
)

type (
	ABTests struct {
		Config     *config.Configuration
		Account    *config.Account
		AccountID  string
		RunMap     map[string]bool
		LogMap     map[string]bool
		LoggedMap  map[string]bool
		PlanLoaded bool
		mu         sync.Mutex
	}
)

func NewABTests(cfg *config.Configuration) *ABTests {
	abTester := ABTests{
		Config: cfg,
		RunMap: make(map[string]bool),
		LogMap: make(map[string]bool),
	}

	return &abTester
}

func (t *ABTests) SetAccount(account *config.Account) {
	t.Account = account
}

func (t *ABTests) SetAccountID(body []byte) {
	t.AccountID = gjson.GetBytes(body, "site.publisher.id").String()
}

func (t *ABTests) Run(module string) bool {
	if !t.PlanLoaded {
		t.planHost()
		t.planAccount()
		t.PlanLoaded = true
	}

	val, ok := t.RunMap[module]
	if !ok {
		return true
	}
	return val
}

func (t *ABTests) WriteOutcome(outcome *StageOutcome) {
	for module, enabled := range t.RunMap {
		if !t.LogMap[module] {
			continue
		}

		if t.getLogged(module) {
			continue
		}

		var a hookanalytics.Activity
		resultStatus := hookanalytics.ResultStatusSkip
		if enabled {
			resultStatus = hookanalytics.ResultStatusRun
		}
		a.Name = "core-module-abtests"
		a.Status = hookanalytics.ActivityStatusSuccess
		a.Results = append(a.Results, hookanalytics.Result{
			Status: resultStatus,
			Values: map[string]interface{}{
				"module": module,
			},
		})

		for groupKey, group := range outcome.Groups {
			for invocationResultKey, invocationResult := range group.InvocationResults {
				if invocationResult.HookID.ModuleCode == module {
					outcome.Groups[groupKey].InvocationResults[invocationResultKey].AnalyticsTags.Activities =
						append(outcome.Groups[groupKey].InvocationResults[invocationResultKey].AnalyticsTags.Activities, a)
					t.setLogged(module)
				}
			}
		}

		if enabled || t.getLogged(module) {
			continue
		}

		var group GroupOutcome
		var invocationResult HookOutcome
		invocationResult.AnalyticsTags.Activities = append(invocationResult.AnalyticsTags.Activities, a)
		invocationResult.Status = StatusSuccess
		invocationResult.HookID.ModuleCode = module
		group.InvocationResults = append(group.InvocationResults, invocationResult)
		outcome.Groups = append(outcome.Groups, group)
		t.setLogged(module)
	}
}

func (t *ABTests) planHost() {
	for _, abtest := range t.Config.Hooks.HostExecutionPlan.ABTests {
		if abtest.ModuleCode == "" {
			glog.Warning("hooks.execution_plan.[]abtests.module_code is required")
			continue
		}

		if abtest.Enabled == nil || !*abtest.Enabled {
			continue
		}

		if !t.containsAccount(abtest.Accounts) {
			t.RunMap[abtest.ModuleCode] = false
			continue
		}

		pa := uint16(100)
		if abtest.PercentActive != nil && *abtest.PercentActive < uint16(100) {
			pa = *abtest.PercentActive
		}
		t.RunMap[abtest.ModuleCode] = uint16(rand.Intn(100)) < pa

		lat := true
		if abtest.LogAnalyticsTag != nil {
			lat = *abtest.LogAnalyticsTag
		}
		t.LogMap[abtest.ModuleCode] = lat

		if lat {
			t.LoggedMap[abtest.ModuleCode] = false
		}
	}
}

func (t *ABTests) planAccount() {
	cfg := t.Config.Hooks.DefaultAccountExecutionPlan.ABTests
	if t.Account != nil {
		cfg = t.Account.Hooks.ExecutionPlan.ABTests
	}

	for _, abtest := range cfg {
		if abtest.ModuleCode == "" {
			glog.Warning("hooks.execution_plan.[]abtests.module_code is required")
			continue
		}

		if abtest.Enabled == nil || !*abtest.Enabled {
			delete(t.RunMap, abtest.ModuleCode)
			continue
		}

		pa := uint16(100)
		if abtest.PercentActive != nil && *abtest.PercentActive < uint16(100) {
			pa = *abtest.PercentActive
		}
		t.RunMap[abtest.ModuleCode] = uint16(rand.Intn(100)) < pa

		lat := true
		if abtest.LogAnalyticsTag != nil {
			lat = *abtest.LogAnalyticsTag
		}
		t.LogMap[abtest.ModuleCode] = lat
	}
}

func (t *ABTests) containsAccount(accounts []string) bool {
	if len(accounts) == 0 {
		return true
	}

	accountID := t.AccountID
	if t.Account != nil {
		accountID = t.Account.ID
	}

	return slices.Contains(accounts, accountID)
}

func (t *ABTests) getLogged(module string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.LoggedMap[module]
}

func (t *ABTests) setLogged(module string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.LoggedMap[module] = true
}
