package rpgo

import (
	"context"
	"io"
	"net/http"
)

// NoticeType — запрашиваемый формат уведомлений (POST /delivery-notices-p61).
type NoticeType string

const (
	NoticeTypeXML NoticeType = "XML"
	NoticeTypePDF NoticeType = "PDF"
)

// NoticeStatus — статус формирования запроса на уведомления.
type NoticeStatus string

const (
	NoticeStatusUnknown    NoticeStatus = "UNKNOWN"
	NoticeStatusInProgress NoticeStatus = "IN_PROGRESS"
	NoticeStatusReady      NoticeStatus = "READY"
	NoticeStatusNoData     NoticeStatus = "NO_DATA"
	NoticeStatusFailed     NoticeStatus = "FAILED"
)

// CreateDeliveryNotices запрашивает уведомления о вручении/возврате (по
// пункту 61 ПОУПС) по списку ШПИ в заданном формате. Формирование
// происходит асинхронно; возвращённый id используется в
// GetDeliveryNoticesStatus и GetDeliveryNoticesContent.
func (c *Client) CreateDeliveryNotices(ctx context.Context, noticeType NoticeType, ids []string) (string, error) {
	data, err := postJSON[struct {
		ID string `json:"id"`
	}](c, ctx, c.path("/delivery-notices-p61"), struct {
		NoticeType NoticeType `json:"noticeType"`
		IDs        []string   `json:"ids"`
	}{noticeType, ids})
	if err != nil {
		return "", err
	}
	return data.ID, nil
}

// GetDeliveryNoticesStatus возвращает статус формирования запроса на
// уведомления, созданного через CreateDeliveryNotices.
func (c *Client) GetDeliveryNoticesStatus(ctx context.Context, id string) (NoticeStatus, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.pathf("/delivery-notices-p61", id, "/status"), nil)
	if err != nil {
		return "", err
	}

	data, err := doJSON[struct {
		Status NoticeStatus `json:"status"`
	}](c, req)
	if err != nil {
		return "", err
	}
	return data.Status, nil
}

// GetDeliveryNoticesContent скачивает zip-архив с XML- или PDF-уведомлениями
// (для XML — с отсоединёнными подписями) после того, как
// GetDeliveryNoticesStatus вернул NoticeStatusReady. Закрыть возвращённый
// reader — обязанность вызывающего кода. Чтение из него привязано к ctx
// запроса: если ctx будет отменён (например, через defer cancel() сразу
// после вызова) раньше, чем тело будет полностью прочитано и закрыто,
// последующие Read вернут ошибку отменённого контекста.
func (c *Client) GetDeliveryNoticesContent(ctx context.Context, id string) (io.ReadCloser, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.pathf("/delivery-notices-p61", id, "/content"), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.doRaw(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
