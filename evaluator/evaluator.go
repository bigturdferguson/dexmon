package evaluator

import (
	"time"

	"dexmon/config"
	"dexmon/types"
)

// AlarmStateReader is the store interface Evaluate needs — allows test mocks.
type AlarmStateReader interface {
	GetAlarmState(account, alarmName, recipient string) (*types.AlarmState, error)
}

// EvalResult is a (alarm, recipient) pair that should be acted upon.
type EvalResult struct {
	AlarmName string
	Recipient string
	Alarm     config.AlarmConfig
}

// Evaluate checks a reading against all alarms and returns:
//   - toFire: recipients that should receive a notification now
//   - toRearm: recipients whose alarm state should be cleared (rearm_on_recovery)
func Evaluate(account string, alarms []config.AlarmConfig, reading types.Reading, store AlarmStateReader, now time.Time) (toFire []EvalResult, toRearm []EvalResult, err error) {
	for _, alarm := range alarms {
		triggered := isTriggered(reading.Value, alarm.Threshold, alarm.Direction) &&
			trendMatches(reading.Trend, alarm.Trend)

		for _, recipient := range alarm.Recipients {
			state, err := store.GetAlarmState(account, alarm.Name, recipient)
			if err != nil {
				return nil, nil, err
			}

			if triggered {
				if shouldFire(alarm, state, now) {
					toFire = append(toFire, EvalResult{
						AlarmName: alarm.Name,
						Recipient: recipient,
						Alarm:     alarm,
					})
				}
			} else if alarm.RearmOnRecovery && (state.LastFiredAt != nil || state.SnoozedUntil != nil) {
				toRearm = append(toRearm, EvalResult{
					AlarmName: alarm.Name,
					Recipient: recipient,
					Alarm:     alarm,
				})
			}
		}
	}
	return toFire, toRearm, nil
}

func isTriggered(value, threshold int, direction string) bool {
	switch direction {
	case "above":
		return value > threshold
	case "below":
		return value < threshold
	default:
		return false
	}
}

func trendMatches(trend types.Trend, allowed []string) bool {
	for _, t := range allowed {
		if string(trend) == t {
			return true
		}
	}
	return false
}

func shouldFire(alarm config.AlarmConfig, state *types.AlarmState, now time.Time) bool {
	if state.SnoozedUntil != nil && now.Before(*state.SnoozedUntil) {
		return false
	}
	if state.ReceiptID != nil && state.ReceiptExpiresAt != nil && now.Before(*state.ReceiptExpiresAt) {
		return false
	}
	if alarm.Priority != "emergency" && alarm.Backoff != "" && state.LastFiredAt != nil {
		backoff, err := time.ParseDuration(alarm.Backoff)
		if err == nil && now.Sub(*state.LastFiredAt) < backoff {
			return false
		}
	}
	return true
}
