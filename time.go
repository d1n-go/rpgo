package rpgo

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// timeLayout — самый полный формат даты-времени из собственных примеров API
// ("2016-05-31T16:34:49.000+03:00"). Time всегда сериализуется в этот
// формат, чтобы исходящие запросы были однозначны, хотя в документированных
// примерах встречаются и более свободные варианты.
const timeLayout = "2006-01-02T15:04:05.000-07:00"

// moscowFixedZone используется при разборе значений времени без указания
// смещения от UTC — собственные примеры API дают такие значения в
// московском времени. У Москвы фиксированное UTC+3 без перехода на летнее
// время с 2014 года, поэтому здесь достаточно фиксированной зоны без
// обращения к системной базе часовых поясов (которая на некоторых
// платформах, особенно Windows, может отсутствовать).
var moscowFixedZone = time.FixedZone("MSK", 3*60*60)

// Time оборачивает time.Time для полей, которые в документации API названы
// "XMLGregorianCalendar" (даты документа, документа получателя и т.п.).
// Сериализуется в timeLayout, а при разборе принимает несколько вариантов
// ISO-8601, поскольку сами примеры в документации их смешивают.
type Time struct {
	time.Time
}

func NewTime(t time.Time) Time { return Time{t} }

func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Format(timeLayout))
}

func (t *Time) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		t.Time = time.Time{}
		return nil
	}

	layouts := []string{
		timeLayout,
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		// xMLGregorianCalendar (заявленный в документации тип этих полей)
		// допускает и просто дату без времени.
		"2006-01-02",
	}
	for _, layout := range layouts {
		// ParseInLocation используется вместо Parse для всех вариантов: если
		// в строке есть явное смещение, оно приоритетнее переданной локации
		// и используется как есть; если смещения нет — иначе Parse молча
		// подставил бы UTC вместо документированного московского времени.
		if parsed, err := time.ParseInLocation(layout, s, moscowFixedZone); err == nil {
			t.Time = parsed
			return nil
		}
	}
	return fmt.Errorf("rpgo: parse time %q", s)
}

// Timestamp хранит unix-время в миллисекундах. В прозе документации
// несколько таких полей (например, acceptanceTimestamp, dateTime в
// access-history) названы String, но в реальных примерах это голые числа в
// JSON; Timestamp на всякий случай умеет разбирать оба варианта.
type Timestamp int64

func (t *Timestamp) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*t = 0
		return nil
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		*t = Timestamp(v)
		return nil
	}
	// число могло прийти как float или в научной нотации (например, после
	// повторной сериализации значения где-то на пути через прокси/шлюз) —
	// ParseInt такое не разбирает, хотя это всё ещё валидный JSON number.
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("rpgo: parse timestamp %q: %w", s, err)
	}
	*t = Timestamp(v)
	return nil
}

// Time переводит unix-время в миллисекундах в time.Time.
func (t Timestamp) Time() time.Time {
	return time.UnixMilli(int64(t))
}
