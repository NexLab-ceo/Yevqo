package control

import (
	"strconv"

	"veltrix/internal/shared"
)

func parseReportPeriodDays(raw string) (int, error) {
	if raw == "" { return 30, nil }
	days, err := strconv.Atoi(raw)
	if err != nil || (days != 7 && days != 30 && days != 90) {
		return 0, shared.ErrBadRequest("report period must be 7, 30, or 90 days")
	}
	return days, nil
}
