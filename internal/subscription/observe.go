package subscription

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Observe returns accounting even when a valid included pool is exhausted.
// Admission and dashboard observation use the same provider parser.
func (u CursorUsage) Observe(now time.Time) (Snapshot, error) {
	s, err := u.Admit("quota-observation", now)
	var admission *AdmissionError
	if err != nil && (!errors.As(err, &admission) || admission.Code != http.StatusTooManyRequests) {
		return s, err
	}
	if !percentage(u.PlanUsage.AutoPercentUsed) || !percentage(u.PlanUsage.APIPercentUsed) {
		return Snapshot{}, unknown()
	}
	reset := s.Windows[0].ResetAt
	s.Windows = []Window{
		{Pool: "auto", Period: "monthly", UsedPercent: *u.PlanUsage.AutoPercentUsed, ResetAt: reset},
		{Pool: "api", Period: "monthly", UsedPercent: *u.PlanUsage.APIPercentUsed, ResetAt: reset},
		{Pool: "total", Period: "monthly", UsedPercent: *u.PlanUsage.TotalPercentUsed, ResetAt: reset},
	}
	s.IncludedRemaining = u.includedRemaining()
	s.AutoBucketModels = append([]string{}, u.AutoBucketModels...)
	return s, nil
}

func FetchCursorSnapshot(ctx context.Context, client *http.Client, token string) (Snapshot, error) {
	var usage CursorUsage
	if err := ReadJSON(ctx, client, http.MethodPost,
		"https://api2.cursor.sh/aiserver.v1.DashboardService/GetCurrentPeriodUsage", token, &usage); err != nil {
		return Snapshot{}, err
	}
	return usage.Observe(time.Now())
}
