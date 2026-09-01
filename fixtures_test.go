package rpgo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestCommonInfoFixtures разбирает в CommonInfo те самые примеры commonInfo,
// что опубликованы в документации API (раздел «Примеры json») — так
// ловятся ошибки в тегах полей, к которым склонна ручная структура.
func TestCommonInfoFixtures(t *testing.T) {
	cases := []struct {
		file string
		want CommonInfo
	}{
		{
			file: "common_info_fine.json",
			want: CommonInfo{
				Type:     MailTypeRegistered,
				MailRank: 0,
				Document: Document{ID: "1234567890", Title: "Постановление 1234567890 от 01.01.2020 01:01:01"},
				Charge:   &Charge{PaymentName: "Штраф за нарушение ПДД", AmountKopecks: 50000, PaymentPeriod: 60},
				Sender:   Sender{Code: "CODE_1071"},
				Recipient: Recipient{
					CountryCodeOKSM: 643,
					Organization:    &Organization{Name: "Тестовый Тестовый ЮЛ ПР 3", INN: "7700000051"},
				},
			},
		},
		{
			file: "common_info_registered_notice.json",
			want: CommonInfo{
				Type:     MailTypeRegistered,
				MailRank: 8,
				Document: Document{ID: "1234567890", Title: "Уведомление 1234567890"},
				Sender:   Sender{Code: "CODE_1071"},
				Recipient: Recipient{
					CountryCodeOKSM: 643,
					Person:          &Person{LastName: "Кузнецов", FirstName: "Кирилл", Patronymic: "Викторович"},
				},
			},
		},
		{
			file: "common_info_regular_notice.json",
			want: CommonInfo{
				Type:     MailTypeRegular,
				MailRank: 8,
				Document: Document{ID: "1234567890", Title: "Уведомление 1234567890"},
				Sender:   Sender{Code: "CODE_1071"},
				Recipient: Recipient{
					CountryCodeOKSM: 643,
					Person:          &Person{LastName: "Кузнецов", FirstName: "Кирилл", Patronymic: "Викторович"},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("testdata", tc.file))
			if err != nil {
				t.Fatal(err)
			}

			var got CommonInfo
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			if got.Type != tc.want.Type {
				t.Errorf("Type = %q, want %q", got.Type, tc.want.Type)
			}
			if got.MailRank != tc.want.MailRank {
				t.Errorf("MailRank = %d, want %d", got.MailRank, tc.want.MailRank)
			}
			if got.Document.ID != tc.want.Document.ID || got.Document.Title != tc.want.Document.Title {
				t.Errorf("Document = %+v, want %+v", got.Document, tc.want.Document)
			}
			if got.Attachments.PDF == nil || got.Attachments.PDF.AttachmentID != "letter.pdf" {
				t.Errorf("Attachments.PDF = %+v, want attachmentId letter.pdf", got.Attachments.PDF)
			}
			if got.Sender.Code != tc.want.Sender.Code {
				t.Errorf("Sender.Code = %q, want %q", got.Sender.Code, tc.want.Sender.Code)
			}
			if got.Recipient.CountryCodeOKSM != tc.want.Recipient.CountryCodeOKSM {
				t.Errorf("Recipient.CountryCodeOKSM = %d, want %d", got.Recipient.CountryCodeOKSM, tc.want.Recipient.CountryCodeOKSM)
			}

			if tc.want.Charge != nil {
				if got.Charge == nil {
					t.Fatal("Charge = nil, want non-nil")
				}
				if got.Charge.PaymentName != tc.want.Charge.PaymentName || got.Charge.AmountKopecks != tc.want.Charge.AmountKopecks {
					t.Errorf("Charge = %+v, want %+v", got.Charge, tc.want.Charge)
				}
			}
			if tc.want.Recipient.Organization != nil {
				if got.Recipient.Organization == nil || got.Recipient.Organization.Name != tc.want.Recipient.Organization.Name {
					t.Errorf("Recipient.Organization = %+v, want %+v", got.Recipient.Organization, tc.want.Recipient.Organization)
				}
			}
			if tc.want.Recipient.Person != nil {
				if got.Recipient.Person == nil || got.Recipient.Person.LastName != tc.want.Recipient.Person.LastName {
					t.Errorf("Recipient.Person = %+v, want %+v", got.Recipient.Person, tc.want.Recipient.Person)
				}
			}

			// round-trip: сериализуем обратно и убеждаемся, что результат
			// снова разбирается без ошибок и сохраняет поля, нужные API.
			out, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var reparsed CommonInfo
			if err := json.Unmarshal(out, &reparsed); err != nil {
				t.Fatalf("re-unmarshal: %v", err)
			}
			if reparsed.Type != got.Type || reparsed.Document.ID != got.Document.ID {
				t.Errorf("round-trip mismatch: got %+v, reparsed %+v", got, reparsed)
			}
		})
	}
}
