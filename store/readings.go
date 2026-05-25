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

func (s *Store) CountReadings(account string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM readings WHERE account = ?`, account).Scan(&count)
	return count, err
}

func (s *Store) GetReadings(account string, since time.Time) ([]types.Reading, error) {
	rows, err := s.db.Query(
		`SELECT value, trend, recorded_at FROM readings
		 WHERE account = ? AND recorded_at >= ?
		 ORDER BY recorded_at ASC`,
		account, since.UTC(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var readings []types.Reading
	for rows.Next() {
		var r types.Reading
		var trend string
		var recordedAt time.Time
		if err := rows.Scan(&r.Value, &trend, &recordedAt); err != nil {
			return nil, err
		}
		r.Account = account
		r.Trend = types.Trend(trend)
		r.RecordedAt = recordedAt.UTC()
		readings = append(readings, r)
	}
	return readings, rows.Err()
}
