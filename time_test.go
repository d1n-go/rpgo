package rpgo

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTimeUnmarshalDateOnly(t *testing.T) {
	var got Time
	if err := json.Unmarshal([]byte(`"2020-02-01"`), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Format("2006-01-02") != "2020-02-01" {
		t.Errorf("got = %v", got.Time)
	}
}

func TestTimeUnmarshalZonelessIsMoscowTime(t *testing.T) {
	var got Time
	if err := json.Unmarshal([]byte(`"2020-01-01T01:01:02"`), &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// 01:01:02 по Москве (UTC+3) — это 2019-12-31T22:01:02Z. Раньше эта
	// строка молча парсилась как UTC, сдвигая момент времени на 3 часа.
	want := time.Date(2019, 12, 31, 22, 1, 2, 0, time.UTC)
	if !got.UTC().Equal(want) {
		t.Errorf("got.UTC() = %v, want %v", got.UTC(), want)
	}
}

func TestTimeMarshalUnmarshalRoundTrip(t *testing.T) {
	original := NewTime(mustParse(t, "2016-05-31T16:34:49.000+03:00"))

	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got Time
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !got.Equal(original.Time) {
		t.Errorf("got = %v, want %v", got.Time, original.Time)
	}
}

func TestTimestampUnmarshalFloatAndScientificNotation(t *testing.T) {
	// ParseInt не берёт число в виде float/научной нотации — это всё ещё
	// валидный JSON number, который мог появиться после пере-сериализации
	// значения где-то на пути через прокси/шлюз.
	cases := []string{
		`1535015817000`,
		`1535015817000.0`,
		`1.535015817e+12`,
	}
	for _, s := range cases {
		var ts Timestamp
		if err := json.Unmarshal([]byte(s), &ts); err != nil {
			t.Errorf("Unmarshal(%s): %v", s, err)
			continue
		}
		if ts.Time().Unix() != 1535015817 {
			t.Errorf("Unmarshal(%s): Time() = %v", s, ts.Time())
		}
	}
}

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	var v Time
	if err := json.Unmarshal([]byte(`"`+s+`"`), &v); err != nil {
		t.Fatal(err)
	}
	return v.Time
}

func TestTimeUnmarshal(t *testing.T) {
	var empty Time
	if err := json.Unmarshal([]byte(`""`), &empty); err != nil || !empty.IsZero() {
		t.Errorf(`Unmarshal(""): time = %v, err = %v; want zero, nil`, empty.Time, err)
	}

	for _, s := range []string{`"not a date"`, `12345`} {
		var v Time
		if err := json.Unmarshal([]byte(s), &v); err == nil {
			t.Errorf("Unmarshal(%s): expected error", s)
		}
	}
}

func TestTimeMarshalZeroIsNull(t *testing.T) {
	b, err := json.Marshal(Time{})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(b) != "null" {
		t.Errorf("Marshal(zero) = %s, want null", b)
	}
}

func TestTimestampUnmarshalEmptyAndInvalid(t *testing.T) {
	for _, s := range []string{`null`, `""`} {
		ts := Timestamp(42)
		if err := json.Unmarshal([]byte(s), &ts); err != nil || ts != 0 {
			t.Errorf("Unmarshal(%s): ts = %d, err = %v; want 0, nil", s, ts, err)
		}
	}

	var ts Timestamp
	if err := json.Unmarshal([]byte(`"yesterday"`), &ts); err == nil {
		t.Error(`Unmarshal("yesterday"): expected error`)
	}
}
