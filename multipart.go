package rpgo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"reflect"
	"strings"
)

// Attachment — одна не-метаданная часть запроса POST /letters: файл
// PDF/XML/изображения или отсоединённая электронная подпись. FieldName
// должен совпадать с соответствующим attachmentId/attachmentSignatureId,
// на который ссылается CommonInfo, переданный в SendLetter (например,
// "letter.pdf", "letter.pdf.sig").
type Attachment struct {
	FieldName string
	Content   io.Reader
}

// buildLettersMultipart собирает multipart-тело запроса POST /letters в
// памяти. Письма, принимаемые ИС ЭПС, ограничены по числу страниц PDF
// самой документацией API (см. коды ошибок 138/142/144), поэтому вложения
// заведомо небольшие — а буферизация даёт запросу заранее известный
// Content-Length и рабочий req.GetBody для повтора при редиректе, чего не
// было бы при потоковой отправке через io.Pipe (chunked-тело без
// возможности повтора хуже переносят строгие шлюзы/прокси).
func buildLettersMultipart(info CommonInfo, attachments []Attachment) (contentType string, body *bytes.Buffer, err error) {
	infoJSON, err := json.Marshal(info)
	if err != nil {
		return "", nil, fmt.Errorf("rpgo: marshal commonInfo: %w", err)
	}

	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)

	if err := writePart(w, "commonInfo", "commonInfo.json", "application/json", bytes.NewReader(infoJSON)); err != nil {
		return "", nil, err
	}

	for _, a := range attachments {
		// все вложения (pdf, png, xml, ЭП) документированы как
		// application/octet-stream — см. «Памятка по подключению клиентов
		// к ИС ЭПС», раздел 6.1, «Типы multi-part блоков».
		if err := writePart(w, a.FieldName, a.FieldName, "application/octet-stream", a.Content); err != nil {
			return "", nil, err
		}
	}

	if err := w.Close(); err != nil {
		return "", nil, fmt.Errorf("rpgo: close multipart writer: %w", err)
	}

	return w.FormDataContentType(), buf, nil
}

// quoteEscaper экранирует "\" и '"' в значении quoted-string ровно так же,
// как это делает net/http/multipart.Writer.CreateFormFile — этот вариант
// (в отличие от Go-строкового %q или mime.FormatMediaType) даёт обычный
// name="..."/filename="..." без перехода на RFC 2231-кодирование
// (name*=utf-8”...) для не-ASCII символов, которое многие серверные
// multipart-парсеры не понимают.
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// isNilReader сообщает, что content либо untyped nil, либо non-nil
// интерфейс с nil-значением внутри (например, забытый (*os.File)(nil)) —
// оба случая роняют io.Copy паникой при попытке вызвать Read.
func isNilReader(content io.Reader) bool {
	if content == nil {
		return true
	}
	v := reflect.ValueOf(content)
	switch v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

// writePart пишет одну multipart-часть с заданными именем поля, именем
// файла (для Content-Disposition) и Content-Type.
func writePart(w *multipart.Writer, fieldName, filename, contentType string, content io.Reader) error {
	// CR/LF нельзя просто заэкранировать внутри quoted-string по правилам
	// MIME — они однозначно означают конец заголовка/части, поэтому имя с
	// переводом строки могло бы внедрить произвольный заголовок или целую
	// новую часть в multipart-поток. Отклоняем такие имена целиком, а не
	// пытаемся их "обезвредить".
	if strings.ContainsAny(fieldName, "\r\n") {
		return fmt.Errorf("rpgo: invalid attachment field name %q: contains CR or LF", fieldName)
	}
	if strings.ContainsAny(filename, "\r\n") {
		return fmt.Errorf("rpgo: invalid attachment file name %q: contains CR or LF", filename)
	}
	if isNilReader(content) {
		return fmt.Errorf("rpgo: attachment %q has a nil Content reader", fieldName)
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeQuotes(fieldName), escapeQuotes(filename)))
	h.Set("Content-Type", contentType)

	part, err := w.CreatePart(h)
	if err != nil {
		return fmt.Errorf("rpgo: create multipart part %q: %w", fieldName, err)
	}
	if _, err := io.Copy(part, content); err != nil {
		return fmt.Errorf("rpgo: write multipart part %q: %w", fieldName, err)
	}
	return nil
}
