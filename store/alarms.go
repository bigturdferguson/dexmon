package store

import (
	"database/sql"
	"errors"
	"time"

	"dexmon/types"
)

func (s *Store) GetAlarmState(account, alarmName, recipient string) (*types.AlarmState, error) {
	state := &types.AlarmState{
		Account:   account,
		AlarmName: alarmName,
		Recipient: recipient,
	}
	var lastFiredAt, snoozedUntil, receiptExpires sql.NullTime
	var rid sql.NullString
	err := s.db.QueryRow(
		`SELECT id, last_fired_at, snoozed_until, receipt_id, receipt_expires_at, rearmed
		 FROM alarm_state WHERE account = ? AND alarm_name = ? AND recipient = ?`,
		account, alarmName, recipient,
	).Scan(&state.ID, &lastFiredAt, &snoozedUntil, &rid, &receiptExpires, &state.Rearmed)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}
	if lastFiredAt.Valid {
		t := lastFiredAt.Time.UTC()
		state.LastFiredAt = &t
	}
	if snoozedUntil.Valid {
		t := snoozedUntil.Time.UTC()
		state.SnoozedUntil = &t
	}
	if rid.Valid {
		state.ReceiptID = &rid.String
	}
	if receiptExpires.Valid {
		t := receiptExpires.Time.UTC()
		state.ReceiptExpiresAt = &t
	}
	return state, nil
}

func (s *Store) GetAlarmStateByReceiptID(receiptID string) (*types.AlarmState, error) {
	var state types.AlarmState
	var lastFiredAt, snoozedUntil, receiptExpires sql.NullTime
	var rid sql.NullString
	err := s.db.QueryRow(
		`SELECT id, account, alarm_name, recipient, last_fired_at, snoozed_until, receipt_id, receipt_expires_at, rearmed
		 FROM alarm_state WHERE receipt_id = ?`,
		receiptID,
	).Scan(&state.ID, &state.Account, &state.AlarmName, &state.Recipient,
		&lastFiredAt, &snoozedUntil, &rid, &receiptExpires, &state.Rearmed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastFiredAt.Valid {
		t := lastFiredAt.Time.UTC()
		state.LastFiredAt = &t
	}
	if snoozedUntil.Valid {
		t := snoozedUntil.Time.UTC()
		state.SnoozedUntil = &t
	}
	if rid.Valid {
		state.ReceiptID = &rid.String
	}
	if receiptExpires.Valid {
		t := receiptExpires.Time.UTC()
		state.ReceiptExpiresAt = &t
	}
	return &state, nil
}

func (s *Store) UpsertAlarmState(state types.AlarmState) error {
	_, err := s.db.Exec(`
		INSERT INTO alarm_state
		    (account, alarm_name, recipient, last_fired_at, snoozed_until, receipt_id, receipt_expires_at, rearmed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account, alarm_name, recipient) DO UPDATE SET
		    last_fired_at      = excluded.last_fired_at,
		    snoozed_until      = excluded.snoozed_until,
		    receipt_id         = excluded.receipt_id,
		    receipt_expires_at = excluded.receipt_expires_at,
		    rearmed            = excluded.rearmed`,
		state.Account, state.AlarmName, state.Recipient,
		nullTime(state.LastFiredAt),
		nullTime(state.SnoozedUntil),
		nullString(state.ReceiptID),
		nullTime(state.ReceiptExpiresAt),
		state.Rearmed,
	)
	return err
}

// UpdateFiredState sets last_fired_at and optionally receipt_id/receipt_expires_at,
// and clears rearmed. Use this instead of UpsertAlarmState when dispatching.
func (s *Store) UpdateFiredState(account, alarmName, recipient string, lastFiredAt time.Time, receiptID *string, receiptExpiresAt *time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO alarm_state (account, alarm_name, recipient, last_fired_at, receipt_id, receipt_expires_at, rearmed)
		VALUES (?, ?, ?, ?, ?, ?, 0)
		ON CONFLICT(account, alarm_name, recipient) DO UPDATE SET
		    last_fired_at      = excluded.last_fired_at,
		    receipt_id         = excluded.receipt_id,
		    receipt_expires_at = excluded.receipt_expires_at,
		    rearmed            = 0`,
		account, alarmName, recipient,
		lastFiredAt.UTC(),
		nullString(receiptID),
		nullTime(receiptExpiresAt),
	)
	return err
}

func (s *Store) RearmAlarm(account, alarmName, recipient string) error {
	_, err := s.db.Exec(
		`UPDATE alarm_state SET rearmed = 1, snoozed_until = NULL
		 WHERE account = ? AND alarm_name = ? AND recipient = ?`,
		account, alarmName, recipient,
	)
	return err
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t.UTC(), Valid: true}
}

func nullString(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func (s *Store) LogAlarmFired(account, alarmName, recipient string, firedAt time.Time, bgValue int) error {
	_, err := s.db.Exec(
		`INSERT INTO alarm_history (account, alarm_name, recipient, fired_at, bg_value)
		 VALUES (?, ?, ?, ?, ?)`,
		account, alarmName, recipient, firedAt.UTC(), bgValue,
	)
	return err
}

func (s *Store) GetAlarmHistory(account string, since time.Time) ([]types.AlarmHistoryEntry, error) {
	rows, err := s.db.Query(
		`SELECT alarm_name, MIN(recipient) AS recipient, fired_at, bg_value
		 FROM alarm_history
		 WHERE account = ? AND fired_at >= ?
		 GROUP BY account, alarm_name, fired_at
		 ORDER BY fired_at DESC`,
		account, since.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]types.AlarmHistoryEntry, 0)
	for rows.Next() {
		var e types.AlarmHistoryEntry
		var firedAt time.Time
		if err := rows.Scan(&e.AlarmName, &e.Recipient, &firedAt, &e.BGValue); err != nil {
			return nil, err
		}
		e.FiredAt = firedAt.UTC()
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
