# rpgo

[![Go Reference](https://pkg.go.dev/badge/github.com/demen1n/rpgo.svg)](https://pkg.go.dev/github.com/demen1n/rpgo)
[![CI](https://github.com/demen1n/rpgo/actions/workflows/ci.yml/badge.svg)](https://github.com/demen1n/rpgo/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Go-клиент для REST API Почты России ИС ЭПС («Электронные заказные письма»):
отправка писем, электронные уведомления о вручении, уведомления по пункту 61
ПОУПС, коды/история доступа к письму, выгрузка реестра ф103.

Смоделирован по документации «Памятка по подключению клиентов к ИС ЭПС»
(актуальная на момент написания версия API — [`v2.6`](https://www.pochta.ru/assets/Pamyatka_po_podklyucheniyu_klientov_k_IS_EPS_147fa5b62a.pdf)). Версии API у Почты
России менялись не раз (`v1.0` → `v2.3` → `v2.6`) без явного анонса — перед
использованием в проде свяжитесь с `ezp.support-r00@russianpost.ru` и
свяжите версию со своим договором; при необходимости переопределите её через
`WithAPIVersion`.

## Важно: этот пакет не реализует ГОСТ TLS/VipNet

Продуктивный и интеграционный контуры ИС ЭПС доступны только по каналу с
ГОСТ-шифрованием (TLS+ГОСТ через КриптоПро/Stunnel) либо через сеть VipNet —
см. документацию, разделы 4–5. `rpgo` не реализует эту часть: клиент говорит
обычным HTTP(S) + Basic-auth на тот `baseURL`, который вы ему дали — как
правило, это локальный порт, на котором Stunnel/VipNet уже терминирует ГОСТ-
канал (например `http://localhost:3080`, как в примерах самой Почты России).

## Установка

```
go get github.com/demen1n/rpgo
```

## Пример: отправка письма

```go
client := rpgo.NewClient("http://localhost:3080", 1071, "your-auth-token")

info := rpgo.CommonInfo{
    Type: rpgo.MailTypeRegistered,
    Attachments: rpgo.Attachments{
        PDF: &rpgo.AttachmentRef{
            AttachmentID:          "letter.pdf",
            AttachmentSignatureID: "letter.pdf.sig",
        },
    },
    Document: rpgo.Document{
        ID:                 "1234567890",
        RegistrationDate:   rpgo.NewTime(time.Now()),
        RegistrationNumber: "1234567890",
        Title:              "Уведомление 1234567890",
    },
    Sender: rpgo.Sender{
        Code:           "CODE_1071",
        Name:           "ООО Ромашка",
        DepartmentCode: "CODE_1071",
        DepartmentName: "ООО Ромашка",
        Address:        "г. Москва, ...",
    },
    Recipient: rpgo.Recipient{
        CountryCodeOKSM: 643,
        Address:         "г Тольятти пр-кт Ленинский д. 1 кв. 2",
        Person: &rpgo.Person{
            FirstName: "Кирилл",
            LastName:  "Кузнецов",
        },
    },
}

requestCode, err := client.SendLetter(ctx, info,
    rpgo.Attachment{FieldName: "letter.pdf", Content: pdfFile},
    rpgo.Attachment{FieldName: "letter.pdf.sig", Content: sigFile},
)
if err != nil {
    // ошибки не-200 приходят как *rpgo.APIError с HTTPStatus/Code/Message
    var apiErr *rpgo.APIError
    if errors.As(err, &apiErr) {
        log.Printf("почта отклонила письмо: %s (code %d)", apiErr.Message, apiErr.Code)
    }
    return err
}

status, err := client.GetLetterStatus(ctx, requestCode)
```

Электронная подпись (`letter.pdf.sig`) в форматe PKCS#7/DER этим пакетом не
формируется — это отдельный, отдельно лицензируемый шаг (обычно через
КриптоПро `cryptcp` или аналог), выполняемый до вызова `SendLetter`.

## Пример: получение уведомлений о вручении (по ШПИ)

```go
id, err := client.CreateDeliveryNotices(ctx, rpgo.NoticeTypeXML, []string{"12345678901"})
// ...
for {
    status, err := client.GetDeliveryNoticesStatus(ctx, id)
    if status == rpgo.NoticeStatusReady {
        break
    }
    // ...
}
zipBody, err := client.GetDeliveryNoticesContent(ctx, id)
defer zipBody.Close()
```

## Покрытие API

- `POST /letters`, `GET /letters/{requestCode}/status`
- `POST /delivery-notices-p61`, `GET .../status`, `GET .../content`
- `POST /electronic-delivery-notices/batch/status`, `GET /electronic-delivery-notices/{mailId}`
- `GET /access-code/{mail_id}`, `GET /access-history/{mail_id}`
- `GET /f103/{date}`
