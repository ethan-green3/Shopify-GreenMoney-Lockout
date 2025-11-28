package internal

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClientNotification represents a single notification from Green's eNotification API.
// We only model the fields that are currently useful for your service.
type ClientNotification struct {
	ID               int64  `xml:"ClientNotification_ID"`
	Message          string `xml:"Message"`
	ClientID         int64  `xml:"Client_ID"`
	EntryClientID    int64  `xml:"EntryClient_ID"`
	TimeAPIViewed    string `xml:"timeApiViewed"`
	TimeEmailViewed  string `xml:"timeEmailViewed"`
	TimePortalViewed string `xml:"timePortalViewed"`
	TimeUpdated      string `xml:"timeUpdated"`
	TimeCreated      string `xml:"timeCreated"`
	Delete           bool   `xml:"delete"`
}

// UnseenNotificationsResult models the XML from the UnseenNotifications method.
type UnseenNotificationsResult struct {
	XMLName            xml.Name             `xml:"UnseenNotificationsResult"`
	Result             string               `xml:"Result"`
	ResultDescription  string               `xml:"ResultDescription"`
	NotificationsCount int                  `xml:"NotificationsCount"`
	Notifications      []ClientNotification `xml:"Notifications>ClientNotification"`
}

// AllNotificationsResult is identical in shape but has a different root element name.
type AllNotificationsResult struct {
	XMLName            xml.Name             `xml:"AllNotificationsResult"`
	Result             string               `xml:"Result"`
	ResultDescription  string               `xml:"ResultDescription"`
	NotificationsCount int                  `xml:"NotificationsCount"`
	Notifications      []ClientNotification `xml:"Notifications>ClientNotification"`
}

// ClearNotificationResult shares the same body as well.
type ClearNotificationResult struct {
	XMLName            xml.Name             `xml:"ClearNotificationResult"`
	Result             string               `xml:"Result"`
	ResultDescription  string               `xml:"ResultDescription"`
	NotificationsCount int                  `xml:"NotificationsCount"`
	Notifications      []ClientNotification `xml:"Notifications>ClientNotification"`
}

// newENotificationForm returns a url.Values with the common auth & delimiter fields set.
func (c *GreenClient) newENotificationForm() (url.Values, error) {
	if c.ClientID == "" || c.APIPassword == "" {
		return nil, fmt.Errorf("GreenClient missing ClientID or APIPassword")
	}
	form := url.Values{}
	form.Set("Client_ID", c.ClientID)
	form.Set("ApiPassword", c.APIPassword)

	// For now we don't use delimited mode, but the API requires the fields to exist.
	form.Set("x_delim_data", "")
	form.Set("x_delim_char", "")

	return form, nil
}

// eNotificationEndpoint builds the full URL for a given eNotification method, like
// "UnseenNotifications" or "AllNotifications".
func (c *GreenClient) eNotificationEndpoint(method string) string {
	base := strings.TrimRight(c.BaseURL, "/")
	// Example: https://cpsandbox.com/eNotification.asmx/UnseenNotifications
	return base + "/eNotification.asmx/" + method
}

// UnseenNotifications returns only those notifications which haven't been viewed via API
// yet and which haven't been marked for deletion.
func (c *GreenClient) UnseenNotifications(ctx context.Context) (*UnseenNotificationsResult, error) {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}

	form, err := c.newENotificationForm()
	if err != nil {
		return nil, err
	}

	endpoint := c.eNotificationEndpoint("UnseenNotifications")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("new UnseenNotifications request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do UnseenNotifications request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("UnseenNotifications non-2xx: %s", resp.Status)
	}

	var result UnseenNotificationsResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode UnseenNotificationsResult: %w", err)
	}

	if result.Result != "0" {
		return &result, fmt.Errorf("UnseenNotifications Result=%s Description=%s", result.Result, result.ResultDescription)
	}

	return &result, nil
}

// AllNotifications returns all notifications in your queue, regardless of whether they
// have previously been viewed or marked for deletion. Mostly useful for debugging.
func (c *GreenClient) AllNotifications(ctx context.Context) (*AllNotificationsResult, error) {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}

	form, err := c.newENotificationForm()
	if err != nil {
		return nil, err
	}

	endpoint := c.eNotificationEndpoint("AllNotifications")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("new AllNotifications request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do AllNotifications request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AllNotifications non-2xx: %s", resp.Status)
	}

	var result AllNotificationsResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode AllNotificationsResult: %w", err)
	}

	if result.Result != "0" {
		return &result, fmt.Errorf("AllNotifications Result=%s Description=%s", result.Result, result.ResultDescription)
	}

	return &result, nil
}

// ClearNotification marks a single notification as deleted so it won't return on subsequent
func (c *GreenClient) ClearNotification(ctx context.Context, notificationID int64) (*ClearNotificationResult, error) {
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}

	form, err := c.newENotificationForm()
	if err != nil {
		return nil, err
	}
	form.Set("ClientNotification_ID", fmt.Sprintf("%d", notificationID))

	endpoint := c.eNotificationEndpoint("ClearNotification")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("new ClearNotification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do ClearNotification request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ClearNotification non-2xx: %s", resp.Status)
	}

	var result ClearNotificationResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode ClearNotificationResult: %w", err)
	}

	if result.Result != "0" {
		return &result, fmt.Errorf("ClearNotification Result=%s Description=%s", result.Result, result.ResultDescription)
	}

	return &result, nil
}
