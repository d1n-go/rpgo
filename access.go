package rpgo

import (
	"context"
	"net/http"
)

// AccessRecord — одна запись в истории доступа к письму.
type AccessRecord struct {
	DateTime Timestamp `json:"dateTime"`
	FullName string    `json:"fullName"`
}

// GetAccessCode возвращает код доступа, напечатанный на бумажном письме и
// используемый получателем для просмотра его онлайн-версии.
func (c *Client) GetAccessCode(ctx context.Context, mailID string) (string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.pathf("/access-code", mailID, ""), nil)
	if err != nil {
		return "", err
	}

	data, err := doJSON[struct {
		AccessCode string `json:"accessCode"`
	}](c, req)
	if err != nil {
		return "", err
	}
	return data.AccessCode, nil
}

// GetAccessHistory возвращает историю доступов к письму по ШПИ.
func (c *Client) GetAccessHistory(ctx context.Context, mailID string) ([]AccessRecord, error) {
	req, err := c.newRequest(ctx, http.MethodGet, c.pathf("/access-history", mailID, ""), nil)
	if err != nil {
		return nil, err
	}

	data, err := doJSON[struct {
		AccessHistory []AccessRecord `json:"accessHistory"`
	}](c, req)
	if err != nil {
		return nil, err
	}
	return data.AccessHistory, nil
}
