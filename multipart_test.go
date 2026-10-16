package rpgo

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestSendLetterPartsHaveFilename(t *testing.T) {
	// без filename= в Content-Disposition часть с PDF рискует быть
	// разобрана сервером как обычное текстовое form-поле, а не файл.
	var dispositions = map[string]string{}

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			t.Fatalf("Content-Type = %q, err = %v", r.Header.Get("Content-Type"), err)
		}

		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			dispositions[part.FormName()] = part.FileName()
		}

		w.Write([]byte(`{"code":200,"data":{"requestCode":"req"}}`))
	})

	info := CommonInfo{
		Type:        MailTypeRegistered,
		Attachments: Attachments{PDF: &AttachmentRef{AttachmentID: "letter.pdf"}},
		Document:    Document{ID: "1", RegistrationNumber: "1"},
		Sender:      Sender{Code: "CODE_1071", Name: "n", DepartmentCode: "CODE_1071", DepartmentName: "n"},
		Recipient:   Recipient{CountryCodeOKSM: 643, Address: "addr", Person: &Person{FirstName: "И", LastName: "Т"}},
	}

	if _, err := c.SendLetter(context.Background(), info, Attachment{FieldName: "letter.pdf", Content: strings.NewReader("data")}); err != nil {
		t.Fatalf("SendLetter: %v", err)
	}

	if dispositions["commonInfo"] == "" {
		t.Errorf("commonInfo part has no filename")
	}
	if dispositions["letter.pdf"] != "letter.pdf" {
		t.Errorf("letter.pdf part filename = %q, want %q", dispositions["letter.pdf"], "letter.pdf")
	}
}

func TestWritePartEscapesSpecialFieldName(t *testing.T) {
	// имя поля с кавычкой должно давать валидный, разбираемый заголовок, а
	// не сломанный Content-Disposition (Go-строковое %q экранирует иначе,
	// чем того требует MIME).
	contentType, body, err := buildLettersMultipart(
		CommonInfo{Type: MailTypeRegular, Document: Document{ID: "1", RegistrationNumber: "1"}, Sender: Sender{Code: "c", Name: "n", DepartmentCode: "c", DepartmentName: "n"}, Recipient: Recipient{CountryCodeOKSM: 643, Address: "a"}},
		[]Attachment{{FieldName: `weird"name.pdf`, Content: strings.NewReader("x")}},
	)
	if err != nil {
		t.Fatalf("buildLettersMultipart: %v", err)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("ParseMediaType: %v", err)
	}

	mr := multipart.NewReader(body, params["boundary"])
	var sawWeird bool
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		if part.FormName() == `weird"name.pdf` {
			sawWeird = true
		}
	}
	if !sawWeird {
		t.Error("part with a quote in its field name was not parsed back correctly")
	}
}

func TestWritePartNonASCIIFieldNameKeepsPlainNameFallback(t *testing.T) {
	// mime.FormatMediaType, отправлявшийся раньше, для не-ASCII значений
	// переходит на RFC 2231 (name*=utf-8''...) и не оставляет обычного
	// name="...", который понимают не все серверные multipart-парсеры.
	_, body, err := buildLettersMultipart(
		CommonInfo{Type: MailTypeRegular, Document: Document{ID: "1", RegistrationNumber: "1"}, Sender: Sender{Code: "c", Name: "n", DepartmentCode: "c", DepartmentName: "n"}, Recipient: Recipient{CountryCodeOKSM: 643, Address: "a"}},
		[]Attachment{{FieldName: "письмо.pdf", Content: strings.NewReader("x")}},
	)
	if err != nil {
		t.Fatalf("buildLettersMultipart: %v", err)
	}

	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(raw), `name="письмо.pdf"`) {
		t.Errorf("multipart body missing plain name=%q fallback:\n%s", "письмо.pdf", raw)
	}
	if strings.Contains(string(raw), "name*=") {
		t.Error("multipart body uses RFC 2231 name*= encoding instead of a plain name=")
	}
}

func TestWritePartRejectsCRLFInFieldName(t *testing.T) {
	// перевод строки в имени поля мог бы внедрить произвольный заголовок
	// или отдельную часть в multipart-поток — такие имена нужно отклонять,
	// а не пытаться экранировать.
	_, _, err := buildLettersMultipart(
		CommonInfo{Type: MailTypeRegular, Document: Document{ID: "1", RegistrationNumber: "1"}, Sender: Sender{Code: "c", Name: "n", DepartmentCode: "c", DepartmentName: "n"}, Recipient: Recipient{CountryCodeOKSM: 643, Address: "a"}},
		[]Attachment{{FieldName: "letter.pdf\r\nX-Injected: evil", Content: strings.NewReader("x")}},
	)
	if err == nil {
		t.Fatal("expected error for a field name containing CRLF, got nil")
	}
}

func TestBuildLettersMultipartNilAttachmentContentReturnsError(t *testing.T) {
	// забытый (нулевой) Attachment.Content раньше падал паникой внутри
	// io.Copy — теперь это обычная ошибка rpgo.
	_, _, err := buildLettersMultipart(
		CommonInfo{Type: MailTypeRegular, Document: Document{ID: "1", RegistrationNumber: "1"}, Sender: Sender{Code: "c", Name: "n", DepartmentCode: "c", DepartmentName: "n"}, Recipient: Recipient{CountryCodeOKSM: 643, Address: "a"}},
		[]Attachment{{FieldName: "letter.pdf"}},
	)
	if err == nil {
		t.Fatal("expected error for a nil Content reader, got nil")
	}
}

func TestBuildLettersMultipartTypedNilAttachmentContentReturnsError(t *testing.T) {
	// в отличие от нетипизированного nil, забытый (*os.File)(nil),
	// присвоенный полю io.Reader, даёт non-nil интерфейс с nil-значением
	// внутри — простое "content == nil" его не ловит, а io.Copy на нём
	// падает паникой.
	var f *os.File
	_, _, err := buildLettersMultipart(
		CommonInfo{Type: MailTypeRegular, Document: Document{ID: "1", RegistrationNumber: "1"}, Sender: Sender{Code: "c", Name: "n", DepartmentCode: "c", DepartmentName: "n"}, Recipient: Recipient{CountryCodeOKSM: 643, Address: "a"}},
		[]Attachment{{FieldName: "letter.pdf", Content: f}},
	)
	if err == nil {
		t.Fatal("expected error for a typed-nil Content reader, got nil")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("disk read failed") }

// valueReader — io.Reader с не-указательным получателем: isNilReader не
// должен принимать его за nil только потому, что reflect не может вызвать
// IsNil на структуре.
type valueReader struct{ s string }

func (r valueReader) Read(p []byte) (int, error) { return copy(p, r.s), io.EOF }

func TestBuildLettersMultipartAttachmentReadError(t *testing.T) {
	_, _, err := buildLettersMultipart(
		CommonInfo{Type: MailTypeRegular},
		[]Attachment{{FieldName: "letter.pdf", Content: failingReader{}}},
	)
	if err == nil || !strings.Contains(err.Error(), "disk read failed") {
		t.Fatalf("err = %v, want wrapped read error", err)
	}
}

func TestBuildLettersMultipartValueTypeReader(t *testing.T) {
	_, body, err := buildLettersMultipart(
		CommonInfo{Type: MailTypeRegular},
		[]Attachment{{FieldName: "letter.pdf", Content: valueReader{"%PDF-1.4"}}},
	)
	if err != nil {
		t.Fatalf("buildLettersMultipart: %v", err)
	}
	if !strings.Contains(body.String(), "%PDF-1.4") {
		t.Error("attachment content missing from multipart body")
	}
}
