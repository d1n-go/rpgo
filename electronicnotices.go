package rpgo

import (
	"context"
	"io"
	"net/http"
)

// ElectronicNoticeStatusCode — статус одного элемента в ответе
// BatchElectronicDeliveryNoticeStatus.
type ElectronicNoticeStatusCode string

const (
	// ElectronicNoticeOK означает, что уведомление доступно для скачивания
	// через GetElectronicDeliveryNotice.
	ElectronicNoticeOK                   ElectronicNoticeStatusCode = "OK"
	ElectronicNoticeNotAvailable         ElectronicNoticeStatusCode = "NOT_AVAILABLE"
	ElectronicNoticeInvalidBarcodeFormat ElectronicNoticeStatusCode = "INVALID_BARCODE_FORMAT"
	ElectronicNoticeForbidden            ElectronicNoticeStatusCode = "FORBIDDEN"
	ElectronicNoticeServiceNotAvailable  ElectronicNoticeStatusCode = "SERVICE_NOT_AVAILABLE"
	ElectronicNoticeLetterNotFound       ElectronicNoticeStatusCode = "LETTER_NOT_FOUND"
	ElectronicNoticeExpired              ElectronicNoticeStatusCode = "EXPIRED"
)

// ElectronicNoticeStatus — одна запись в ответе BatchElectronicDeliveryNoticeStatus.
type ElectronicNoticeStatus struct {
	MailID string                     `json:"mailId"`
	Status ElectronicNoticeStatusCode `json:"status"`
}

// BatchElectronicDeliveryNoticeStatus возвращает статусы электронных
// уведомлений о вручении по пакету ШПИ.
func (c *Client) BatchElectronicDeliveryNoticeStatus(ctx context.Context, mailIDs []string) ([]ElectronicNoticeStatus, error) {
	data, err := postJSON[struct {
		Statuses []ElectronicNoticeStatus `json:"statuses"`
	}](c, ctx, c.path("/electronic-delivery-notices/batch/status"), struct {
		MailIDs []string `json:"mailIds"`
	}{mailIDs})
	if err != nil {
		return nil, err
	}
	return data.Statuses, nil
}

// GetElectronicDeliveryNotice скачивает скан электронного уведомления о
// вручении для ШПИ со статусом ElectronicNoticeOK. Возвращает тело ответа
// (вызывающий код обязан его закрыть) и его Content-Type, который в
// документации API указан как "application/pdf" либо "image/jpeg". Чтение
// из тела привязано к ctx запроса: если ctx будет отменён раньше, чем тело
// будет полностью прочитано и закрыто, последующие Read вернут ошибку
// отменённого контекста.
func (c *Client) GetElectronicDeliveryNotice(ctx context.Context, mailID string) (body io.ReadCloser, contentType string, err error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.pathf("/electronic-delivery-notices", mailID, ""), nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := c.doRaw(req)
	if err != nil {
		return nil, "", err
	}

	return resp.Body, resp.Header.Get("Content-Type"), nil
}
