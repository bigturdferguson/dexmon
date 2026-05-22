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
		`SELECT id, last_fired_at, snoozed_until, receipt_id, receipt_expires_at
		 FROM alarm_state WHERE account = ? AND alarm_name = ? AND recipient = ?`,
		account, alarmName, recipient,
	).Scan(&state.ID, &lastFiredAt, &snoozedUntil, &rid, &receiptExpires)
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
		`SELECT id, account, alarm_name, recipient, last_fired_at, snoozed_until, receipt_id, receipt_expires_at
		 FROM alarm_state WHERE receipt_id = ?`,
		receiptID,
	).Scan(&state.ID, &state.Account, &state.AlarmName, &state.Recipient,
		&lastFiredAt, &snoozedUntil, &rid, &receiptExpires)
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
		    (account, alarm_name, recipient, last_fired_at, snoozed_until, receipt_id, receipt_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(account, alarm_name, recipient) DO UPDATE SET
		    last_fired_at      = excluded.last_fired_at,
		    snoozed_until      = excluded.snoozed_until,
		    receipt_id         = excluded.receipt_id,
		    receipt_expires_at = excluded.receipt_expires_at`,
		state.Account, state.AlarmName, state.Recipient,
		nullTime(state.LastFiredAt),
		nullTime(state.SnoozedUntil),
		nullString(state.ReceiptID),
		nullTime(state.ReceiptExpiresAt),
	)
	return err
}

func (s *Store) ClearAlarmRearm(account, alarmName, recipient string) error {
	_, err := s.db.Exec(
		`UPDATE alarm_state SET last_fired_at = NULL, snoozed_until = NULL
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
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}
