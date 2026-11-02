// Package store persists raw samples and minute OHLC bars in pure Go SQLite.
package store

import (
	"database/sql"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/metrics"
	_ "modernc.org/sqlite"
)

type Store struct {
	db             *sql.DB
	insert, candle *sql.Stmt
}

func Open(path string) (_ *Store, err error) {
	dsn := path
	if path != ":memory:" {
		abs, e := filepath.Abs(path)
		if e != nil {
			return nil, e
		}
		if e = os.MkdirAll(filepath.Dir(abs), 0700); e != nil {
			return nil, e
		}
		f, e := os.OpenFile(abs, os.O_CREATE|os.O_RDWR, 0600)
		if e != nil {
			return nil, e
		}
		if e = f.Close(); e != nil {
			return nil, e
		}
		dsn = (&url.URL{Scheme: "file", Path: abs}).String()
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	db.SetMaxOpenConns(1)
	defer func() {
		if err != nil {
			s.Close()
		}
	}()
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return nil, err
	}
	if version > 1 {
		return nil, fmt.Errorf("database schema %d is newer than supported version 1", version)
	}
	const schema = `PRAGMA busy_timeout=5000;
PRAGMA auto_vacuum=INCREMENTAL;
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA cache_size=-4096;
CREATE TABLE IF NOT EXISTS samples (
 ts INTEGER PRIMARY KEY, cpu REAL, memory REAL, swap REAL,
 disk_read REAL, disk_write REAL, net_rx REAL, net_tx REAL,
 load1 REAL, load5 REAL, load15 REAL
);
CREATE TABLE IF NOT EXISTS candles (
 metric INTEGER NOT NULL, bucket INTEGER NOT NULL,
 open REAL NOT NULL, high REAL NOT NULL, low REAL NOT NULL, close REAL NOT NULL,
 count INTEGER NOT NULL, first_ts INTEGER NOT NULL, last_ts INTEGER NOT NULL,
 PRIMARY KEY(metric,bucket)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS candles_time ON candles(bucket);
PRAGMA user_version=1;`
	if _, err = db.Exec(schema); err != nil {
		return nil, fmt.Errorf("initialize SQLite: %w", err)
	}
	s.insert, err = db.Prepare("INSERT INTO samples VALUES(?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(ts) DO NOTHING")
	if err != nil {
		return nil, err
	}
	s.candle, err = db.Prepare(`INSERT INTO candles VALUES(?,?,?,?,?,?,1,?,?)
ON CONFLICT(metric,bucket) DO UPDATE SET
 open=CASE WHEN excluded.first_ts<candles.first_ts THEN excluded.open ELSE candles.open END,
 high=MAX(candles.high,excluded.high), low=MIN(candles.low,excluded.low),
 close=CASE WHEN excluded.last_ts>candles.last_ts THEN excluded.close ELSE candles.close END,
 count=candles.count+1, first_ts=MIN(candles.first_ts,excluded.first_ts),
 last_ts=MAX(candles.last_ts,excluded.last_ts)`)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s.insert != nil {
		s.insert.Close()
	}
	if s.candle != nil {
		s.candle.Close()
	}
	return s.db.Close()
}

func (s *Store) Insert(sample metrics.Sample) error {
	if sample.Time.IsZero() || sample.Time.UnixMilli() < 0 {
		return fmt.Errorf("invalid sample timestamp")
	}
	for i, v := range sample.Values {
		if sample.Unavailable[i] {
			continue
		}
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return fmt.Errorf("invalid metric value %v", v)
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	args := []any{sample.Time.UnixMilli()}
	for i, v := range sample.Values {
		if sample.Unavailable[i] {
			args = append(args, nil)
		} else {
			args = append(args, v)
		}
	}
	r, err := tx.Stmt(s.insert).Exec(args...)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return tx.Commit()
	}
	stmt := tx.Stmt(s.candle)
	for i, v := range sample.Values {
		if sample.Unavailable[i] {
			continue
		}
		if _, err = stmt.Exec(i, sample.Time.Unix()/60*60, v, v, v, v, sample.Time.UnixMilli(), sample.Time.UnixMilli()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func valid(metric int, limit int) error {
	if metric < 0 || metric >= metrics.Count {
		return fmt.Errorf("invalid metric")
	}
	if limit < 1 || limit > 4096 {
		return fmt.Errorf("limit must be 1..4096")
	}
	return nil
}

// Candles returns actual bars in the limit-period window ending at end's bucket.
// Missing periods remain gaps; the current bucket is a partial candle.
func (s *Store) Candles(metric int, period time.Duration, end time.Time, limit int) ([]metrics.Candle, error) {
	if err := valid(metric, limit); err != nil {
		return nil, err
	}
	found := false
	for _, p := range metrics.Periods {
		if p == period {
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("unsupported candle period")
	}
	seconds := int64(period / time.Second)
	last := end.Unix() / seconds * seconds
	first := last - int64(limit-1)*seconds
	if period < time.Minute {
		return s.secondCandles(metric, seconds, first, last+seconds)
	}
	rows, err := s.db.Query(`WITH grouped AS (
 SELECT (bucket / ?) * ? AS t, MIN(bucket) AS first, MAX(bucket) AS last,
 MAX(high) AS h, MIN(low) AS l, SUM(count) AS n FROM candles
 WHERE metric=? AND bucket>=? AND bucket<? GROUP BY t
) SELECT t,
 (SELECT open FROM candles WHERE metric=? AND bucket=grouped.first),h,l,
 (SELECT close FROM candles WHERE metric=? AND bucket=grouped.last),n
 FROM grouped ORDER BY t`, seconds, seconds, metric, first, last+seconds, metric, metric)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []metrics.Candle
	for rows.Next() {
		var b metrics.Candle
		var ts int64
		if err := rows.Scan(&ts, &b.Open, &b.High, &b.Low, &b.Close, &b.Count); err != nil {
			return nil, err
		}
		b.Time = time.Unix(ts, 0).UTC()
		out = append(out, b)
	}
	return out, rows.Err()
}

// Second bars require retained raw samples; minute OHLC cannot reconstruct them.
func (s *Store) secondCandles(metric int, seconds, first, end int64) ([]metrics.Candle, error) {
	column := metrics.All[metric].Key
	query := `WITH grouped AS (
 SELECT ts / ? * ? AS t, MIN(ts) AS first, MAX(ts) AS last,
 MAX(` + column + `) AS h, MIN(` + column + `) AS l, COUNT(*) AS n
 FROM samples WHERE ts>=? AND ts<? AND ` + column + ` IS NOT NULL GROUP BY t
) SELECT t,
 (SELECT ` + column + ` FROM samples WHERE ts=grouped.first),h,l,
 (SELECT ` + column + ` FROM samples WHERE ts=grouped.last),n FROM grouped ORDER BY t`
	rows, err := s.db.Query(query, seconds*1000, seconds, first*1000, end*1000)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []metrics.Candle
	for rows.Next() {
		var b metrics.Candle
		var ts int64
		if err := rows.Scan(&ts, &b.Open, &b.High, &b.Low, &b.Close, &b.Count); err != nil {
			return nil, err
		}
		b.Time = time.Unix(ts, 0).UTC()
		out = append(out, b)
	}
	return out, rows.Err()
}

// Points selects the last sample row in each time bin, leaving a gap when that
// row's metric is unavailable. Each bin does one timestamp index seek, even for
// many days of missing values, rather than scanning backwards through NULLs.
func (s *Store) Points(metric int, start, end time.Time, limit int) ([]metrics.Point, error) {
	if err := valid(metric, limit); err != nil {
		return nil, err
	}
	span := end.UnixMilli() - start.UnixMilli()
	if span <= 0 {
		return nil, fmt.Errorf("invalid point time range")
	}
	column := metrics.All[metric].Key // Strict metric index above; never user SQL.
	query := `WITH RECURSIVE bins(i) AS (
 VALUES(0) UNION ALL SELECT i+1 FROM bins WHERE i+1 < ?
) SELECT samples.ts, samples.` + column + ` FROM bins JOIN samples ON samples.ts=(
 SELECT ts FROM samples WHERE ts >= ? + i * ? / ?
 AND ts < ? + (i+1) * ? / ? + CASE WHEN i = ? THEN 1 ELSE 0 END
 ORDER BY ts DESC LIMIT 1
) WHERE samples.` + column + ` IS NOT NULL ORDER BY samples.ts`
	rows, err := s.db.Query(query, limit, start.UnixMilli(), span, limit, start.UnixMilli(), span, limit, limit-1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []metrics.Point
	for rows.Next() {
		var p metrics.Point
		var ts int64
		if err := rows.Scan(&ts, &p.Value); err != nil {
			return nil, err
		}
		p.Time = time.UnixMilli(ts).UTC()
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) Cleanup(now time.Time, rawDays, historyDays int) error {
	if rawDays < 1 || historyDays < rawDays {
		return fmt.Errorf("invalid retention")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("DELETE FROM samples WHERE ts < ?", now.Add(-time.Duration(rawDays)*24*time.Hour).UnixMilli()); err != nil {
		return err
	}
	// Retain the boundary minute in full rather than cut an OHLC candle in half.
	cutoff := now.Add(-time.Duration(historyDays)*24*time.Hour).Unix() / 60 * 60
	if _, err = tx.Exec("DELETE FROM candles WHERE bucket < ?", cutoff); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	_, err = s.db.Exec("PRAGMA incremental_vacuum(256)")
	return err
}
