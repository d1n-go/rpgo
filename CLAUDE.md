# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Что это

`rpgo` — Go-библиотека (один пакет `rpgo` в корне, без внешних зависимостей, Go 1.23) — клиент REST API Почты России ИС ЭПС («Электронные заказные письма»). Источник истины по API — PDF «Памятка по подключению клиентов к ИС ЭПС» (`Pamyatka_po_podklyucheniyu_klientov_k_IS_EPS_147fa5b62a.pdf` в корне, ссылка в README; удобно читать через `pdftotext -layout`). Описание эндпоинтов — раздел 6, коды ошибок обработки письма — 6.2, рекомендуемые интервалы опроса статусов — 7.6. Сама памятка непоследовательна: методы описаны для `v2.6`, а примеры проверки канала в 6.12 всё ещё ходят на `v2.3`.

Пакет **не** реализует ГОСТ TLS/VipNet: он говорит обычным HTTP(S)+Basic auth на `baseURL`, где туннель уже терминирован (обычно Stunnel на `http://localhost:3080`). Исключение — интеграционный контур с обычным RSA TLS (`https://zakaznoe-async.test.russianpost.ru`, раздел 6.11.2), туда можно ходить напрямую. Электронную подпись (PKCS#7) пакет тоже не формирует.

Комментарии в коде, doc-комментарии, README и сообщения коммитов — на русском. Тексты ошибок — на английском с префиксом `rpgo: `.

Коммиты и PR — без атрибуции Claude: не добавлять `Co-Authored-By: Claude ...`, «Generated with Claude Code» и подобные строки.

## Команды

То же, что проверяет CI (`.github/workflows/ci.yml`):

```sh
gofmt -l .                 # должно быть пусто, иначе CI падает
go vet ./...
go test ./... -race
go test -run TestAPIError ./...   # один тест
```

## Архитектура

- `client.go` — ядро: `Client`, функциональные опции (`WithHTTPClient`, `WithAPIVersion`), Basic-авторизация из `customerID:authToken`, и три хелпера, через которые идут **все** эндпоинты:
  - `doJSON[T]` — читает конверт `{code, message, data}` и возвращает `data`. Ошибкой считается и не-2xx HTTP-статус, и не-2xx `code` внутри конверта при HTTP 2xx. Пустое тело на 2xx — не ошибка.
  - `postJSON[T]` — маршалит payload и вызывает `doJSON`.
  - `doRaw` — для бинарных эндпоинтов (содержимое уведомлений, ф103): отдаёт `*http.Response` потоком, `Body` закрывает вызывающий код. Ответ 2xx с `Content-Type: application/json` трактуется как ошибка.
- Пути строятся только через `c.path(suffix)` (статический суффикс) или `c.pathf(prefix, id, suffix)` (сам экранирует `id` через `url.PathEscape`). Версия API — сегмент пути `/api/<version>/...`.
- Каждая группа эндпоинтов — отдельный файл (`letters.go`, `deliverynotices.go`, `electronicnotices.go`, `access.go`, `f103.go`) с тонкими методами `Client` поверх хелперов. Общие DTO — в `types.go`.
- `errors.go` — `*APIError` (`HTTPStatus`, `Code`, `Message`, сырой `Body`); все неуспешные ответы возвращаются как он.
- `multipart.go` — тело `POST /letters` собирается в памяти (ради Content-Length и повторяемости при редиректе): JSON-часть `commonInfo`, затем вложения как `application/octet-stream`. `Attachment.FieldName` должен совпадать с `attachmentId`/`attachmentSignatureId` в `CommonInfo`. Имена с CR/LF и nil-ридеры отклоняются.
- `time.go` — `Time` (поля «XMLGregorianCalendar»: сериализуется в `2006-01-02T15:04:05.000-07:00`, при разборе принимает несколько форматов, без смещения — московское время UTC+3) и `Timestamp` (unix-миллисекунды, принимает число или строку).
- Чтение тел ответов в память ограничено `maxResponseBodyBytes`; дефолтный HTTP-клиент ставит таймауты на фазы соединения (dial/TLS/заголовки ответа), а не на весь запрос — чтобы не обрывать медленную передачу больших писем.

## Тесты

Тесты поднимают `httptest.Server` через `newTestClient` (`client_test.go`), customerID `1071`, токен `test-token`. `fixtures_test.go` разбирает в `CommonInfo` примеры JSON из документации, лежащие в `testdata/` — при изменении тегов полей в `types.go` проверяйте их.
