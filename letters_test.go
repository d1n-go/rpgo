package rpgo

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func TestSendLetter(t *testing.T) {
	const wantPDF = "%PDF-1.4 fake content"
	const wantSig = "fake signature bytes"

	var gotPath, gotMethod string
	var parts = map[string]string{}
	var commonInfoRaw string

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method

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
			content, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("read part %q: %v", part.FormName(), err)
			}
			if part.FormName() == "commonInfo" {
				commonInfoRaw = string(content)
			} else {
				parts[part.FormName()] = string(content)
			}
		}

		w.Write([]byte(`{"code":200,"data":{"requestCode":"req_089aebf2-a95c-4b93-88b9-8c391137249c"}}`))
	})

	info := CommonInfo{
		Type: MailTypeRegistered,
		Attachments: Attachments{
			PDF: &AttachmentRef{AttachmentID: "letter.pdf", AttachmentSignatureID: "letter.pdf.sig"},
		},
		Document: Document{ID: "1", RegistrationNumber: "1"},
		Sender:   Sender{Code: "CODE_1071", Name: "n", DepartmentCode: "CODE_1071", DepartmentName: "n"},
		Recipient: Recipient{
			CountryCodeOKSM: 643,
			Address:         "some address",
			Person:          &Person{FirstName: "Кирилл", LastName: "Кузнецов"},
		},
	}

	requestCode, err := c.SendLetter(context.Background(), info,
		Attachment{FieldName: "letter.pdf", Content: strings.NewReader(wantPDF)},
		Attachment{FieldName: "letter.pdf.sig", Content: strings.NewReader(wantSig)},
	)
	if err != nil {
		t.Fatalf("SendLetter: %v", err)
	}

	if requestCode != "req_089aebf2-a95c-4b93-88b9-8c391137249c" {
		t.Errorf("requestCode = %q", requestCode)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v2.6/letters" {
		t.Errorf("method/path = %s %s", gotMethod, gotPath)
	}
	if !strings.Contains(commonInfoRaw, `"type":"REGISTERED"`) {
		t.Errorf("commonInfo part = %q, missing type", commonInfoRaw)
	}
	if parts["letter.pdf"] != wantPDF {
		t.Errorf("letter.pdf part = %q, want %q", parts["letter.pdf"], wantPDF)
	}
	if parts["letter.pdf.sig"] != wantSig {
		t.Errorf("letter.pdf.sig part = %q, want %q", parts["letter.pdf.sig"], wantSig)
	}
}

func TestGetLetterStatus(t *testing.T) {
	var gotPath string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{
			"code": 200,
			"data": {
				"requestCode": "req_089aebf2-a95c-4b93-88b9-8c391137249c",
				"type": "ONLINE_DELIVERED",
				"mailId": "20050426117015",
				"acceptanceTimestamp": 1535015817000
			}
		}`))
	})

	status, err := c.GetLetterStatus(context.Background(), "req_089aebf2-a95c-4b93-88b9-8c391137249c")
	if err != nil {
		t.Fatalf("GetLetterStatus: %v", err)
	}

	if want := "/api/v2.6/letters/req_089aebf2-a95c-4b93-88b9-8c391137249c/status"; gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
	if status.Status != LetterStatusOnlineDelivered {
		t.Errorf("Status = %q", status.Status)
	}
	if status.MailID != "20050426117015" {
		t.Errorf("MailID = %q", status.MailID)
	}
	if status.AcceptanceTimestamp.Time().Unix() != 1535015817 {
		t.Errorf("AcceptanceTimestamp = %v", status.AcceptanceTimestamp.Time())
	}
}
