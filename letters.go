package rpgo

import (
	"context"
	"net/http"
)

// LetterStatusType — поле "type" в ответе о статусе письма.
type LetterStatusType string

const (
	LetterStatusUnknown          LetterStatusType = "UNKNOWN"
	LetterStatusAccepted         LetterStatusType = "ACCEPTED"
	LetterStatusInProgress       LetterStatusType = "IN_PROGRESS"
	LetterStatusOnlineDelivered  LetterStatusType = "ONLINE_DELIVERED"
	LetterStatusDeliveredToPrint LetterStatusType = "DELIVERED_TO_PRINT"
	LetterStatusFailed           LetterStatusType = "FAILED"
)

// LetterStatus — разобранный ответ GET /letters/{requestCode}/status.
type LetterStatus struct {
	RequestCode string           `json:"requestCode"`
	Status      LetterStatusType `json:"type"`
	Description string           `json:"description,omitempty"`
	// mailID (ШПИ для заказных писем, UUID для простых) заполняется, как
	// только Status достигает LetterStatusOnlineDelivered или
	// letterStatusDeliveredToPrint.
	MailID string `json:"mailId,omitempty"`
	// AcceptanceTimestamp заполняется одновременно с MailID.
	AcceptanceTimestamp Timestamp `json:"acceptanceTimestamp,omitempty"`
	// ErrorCode заполняется, когда Status равен LetterStatusFailed; значения
	// см. в таблице кодов ошибок в документации API.
	ErrorCode int `json:"errorCode,omitempty"`
}

type sendLetterData struct {
	RequestCode string `json:"requestCode"`
}

// SendLetter отправляет письмо через POST /letters как multipart-запрос:
// JSON-часть "commonInfo", собранная из info, за которой следуют переданные
// attachments (PDF/XML/изображения и их отсоединённые подписи). Возвращает
// присвоенный requestCode, который затем опрашивается через GetLetterStatus.
//
// Значения FieldName в attachments должны совпадать с attachmentId/
// attachmentSignatureId, на которые ссылается info.Attachments.
func (c *Client) SendLetter(ctx context.Context, info CommonInfo, attachments ...Attachment) (string, error) {
	contentType, body, err := buildLettersMultipart(info, attachments)
	if err != nil {
		return "", err
	}

	req, err := c.newRequest(ctx, http.MethodPost, c.path("/letters"), body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)

	data, err := doJSON[sendLetterData](c, req)
	if err != nil {
		return "", err
	}
	return data.RequestCode, nil
}

// GetLetterStatus возвращает текущий статус обработки письма, ранее
// отправленного через SendLetter, по его requestCode.
func (c *Client) GetLetterStatus(ctx context.Context, requestCode string) (*LetterStatus, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.pathf("/letters", requestCode, "/status"), nil)
	if err != nil {
		return nil, err
	}

	data, err := doJSON[LetterStatus](c, req)
	if err != nil {
		return nil, err
	}
	return &data, nil
}
