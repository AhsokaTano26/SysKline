package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/AhsokaTano26/SysKline/internal/metrics"
)

// AllCandles reads all metrics in one SQLite snapshot. Second candles share a
// single raw scan, while long periods share a single minute-candle rollup.
func (s *Store) AllCandles(period time.Duration, end time.Time, limit int) ([metrics.Count][]metrics.Candle, error) {
	var result [metrics.Count][]metrics.Candle
	if err := valid(0, limit); err != nil {
		return result, err
	}
	found := false
	for _, p := range metrics.Periods {
		if p == period {
			found = true
			break
		}
	}
	if !found {
		return result, fmt.Errorf("unsupported candle period")
	}
	seconds := int64(period / time.Second)
	last := end.Unix() / seconds * seconds
	first := last - int64(limit-1)*seconds
	if period < time.Minute {
		return s.allSecondCandles(seconds, first, last+seconds)
	}
	rows, err := s.db.Query(`WITH grouped AS (
 SELECT metric, bucket / ? * ? AS t, MIN(bucket) AS first, MAX(bucket) AS last,
 MAX(high) AS h, MIN(low) AS l, SUM(count) AS n FROM candles
 WHERE bucket>=? AND bucket<? GROUP BY metric,t
) SELECT g.metric,g.t,
 (SELECT open FROM candles c WHERE c.metric=g.metric AND c.bucket=g.first),g.h,g.l,
 (SELECT close FROM candles c WHERE c.metric=g.metric AND c.bucket=g.last),g.n
 FROM grouped g ORDER BY g.metric,g.t`, seconds, seconds, first, last+seconds)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var metric int
		var ts int64
		var b metrics.Candle
		if err := rows.Scan(&metric, &ts, &b.Open, &b.High, &b.Low, &b.Close, &b.Count); err != nil {
			return result, err
		}
		if metric < 0 || metric >= metrics.Count {
			return result, fmt.Errorf("unknown metric %d in history", metric)
		}
		b.Time = time.Unix(ts, 0).UTC()
		result[metric] = append(result[metric], b)
	}
	return result, rows.Err()
}

type nullableSample struct {
	ts     int64
	values [metrics.Count]sql.NullFloat64
}

func (s *nullableSample) targets() []any {
	dest := make([]any, metrics.Count+1)
	dest[0] = &s.ts
	for i := range s.values {
		dest[i+1] = &s.values[i]
	}
	return dest
}

func sampleFields(prefix string) string {
	fields := make([]string, metrics.Count+1)
	fields[0] = prefix + "ts"
	for i, m := range metrics.All {
		fields[i+1] = prefix + m.Key
	}
	return strings.Join(fields, ",")
}

func (s *Store) allSecondCandles(seconds, first, end int64) ([metrics.Count][]metrics.Candle, error) {
	var result [metrics.Count][]metrics.Candle
	rows, err := s.db.Query("SELECT "+sampleFields("")+" FROM samples WHERE ts>=? AND ts<? ORDER BY ts", first*1000, end*1000)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	var sample nullableSample
	dest := sample.targets()
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return result, err
		}
		bucket := time.Unix(sample.ts/(seconds*1000)*seconds, 0).UTC()
		for metric, value := range sample.values {
			if !value.Valid {
				continue
			}
			bars := result[metric]
			v := value.Float64
			if len(bars) == 0 || !bars[len(bars)-1].Time.Equal(bucket) {
				result[metric] = append(bars, metrics.Candle{Time: bucket, Open: v, High: v, Low: v, Close: v, Count: 1})
				continue
			}
			bar := &bars[len(bars)-1]
			bar.High = max(bar.High, v)
			bar.Low = min(bar.Low, v)
			bar.Close = v
			bar.Count++
		}
	}
	return result, rows.Err()
}

// AllPoints does one indexed seek per time bin and reads every metric from the
// selected sample. NULL values remain gaps without scanning older rows for a fill.
func (s *Store) AllPoints(start, end time.Time, limit int) ([metrics.Count][]metrics.Point, error) {
	var result [metrics.Count][]metrics.Point
	if err := valid(0, limit); err != nil {
		return result, err
	}
	span := end.UnixMilli() - start.UnixMilli()
	if span <= 0 {
		return result, fmt.Errorf("invalid point time range")
	}
	query := `WITH RECURSIVE bins(i) AS (
 VALUES(0) UNION ALL SELECT i+1 FROM bins WHERE i+1 < ?
) SELECT ` + sampleFields("samples.") + ` FROM bins JOIN samples ON samples.ts=(
 SELECT ts FROM samples WHERE ts >= ? + i * ? / ?
 AND ts < ? + (i+1) * ? / ? + CASE WHEN i = ? THEN 1 ELSE 0 END
 ORDER BY ts DESC LIMIT 1
) ORDER BY samples.ts`
	rows, err := s.db.Query(query, limit, start.UnixMilli(), span, limit, start.UnixMilli(), span, limit, limit-1)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	var sample nullableSample
	dest := sample.targets()
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return result, err
		}
		at := time.UnixMilli(sample.ts).UTC()
		for metric, value := range sample.values {
			if value.Valid {
				result[metric] = append(result[metric], metrics.Point{Time: at, Value: value.Float64})
			}
		}
	}
	return result, rows.Err()
}
