package types

import "time"

type Trend string

const (
	TrendDoubleUp       Trend = "double_up"
	TrendSingleUp       Trend = "single_up"
	TrendFortyFiveUp    Trend = "forty_five_up"
	TrendFlat           Trend = "flat"
	TrendFortyFiveDown  Trend = "forty_five_down"
	TrendSingleDown     Trend = "single_down"
	TrendDoubleDown     Trend = "double_down"
	TrendNotComputable  Trend = "not_computable"
	TrendRateOutOfRange Trend = "rate_out_of_range"
	TrendNone           Trend = "none"
)

type Reading struct {
	Account    string
	Value      int
	Trend      Trend
	RecordedAt time.Time
}

type AlarmState struct {
	ID               int64
	Account          string
	AlarmName        string
	Recipient        string
	LastFiredAt      *time.Time
	SnoozedUntil     *time.Time
	ReceiptID        *string
	ReceiptExpiresAt *time.Time
}
