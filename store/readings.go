package store

import (
	"time"

	"dexmon/types"
)

func (s *Store) InsertReading(r types.Reading) error {
	_, err := s.db.Exec(
		`INSERT INTO readings (account, value, trend, recorded_at) VALUES (?, ?, ?, ?)`,
		r.Account, r.Value, string(r.Trend), r.RecordedAt.UTC(),
	)
	return err
}

func (s *Store) HasReading(account string, recordedAt time.Time) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM readings WHERE account = ? AND recorded_at = ?`,
		account, recordedAt.UTC(),
	).Scan(&count)
	return count > 0, err
}

func (s *Store) PruneReadings(account string, before time.Time) error {
	_, err := s.db.Exec(
		`DELETE FROM readings WHERE account = ? AND recorded_at < ?`,
		account, before.UTC(),
	)
	return err
}
