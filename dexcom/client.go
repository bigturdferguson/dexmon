package dexcom

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dexmon/types"
)

const (
	defaultBase = "https://share2.dexcom.com/ShareWebServices/Services"
	appID       = "d89443d2-327c-4a6f-89e5-496bbb0317db"
)

var ErrSessionExpired = errors.New("dexcom: session expired")

type Client struct {
	username  string
	password  string
	base      string
	sessionID string
	http      *http.Client
}

func New(username, password string) *Client {
	return NewWithBase(username, password, defaultBase)
}

func NewWithBase(username, password, base string) *Client {
	return &Client{
		username: username,
		password: password,
		base:     base,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) HasSession() bool {
	return c.sessionID != ""
}

func (c *Client) Login() error {
	body, _ := json.Marshal(map[string]string{
		"accountName":   c.username,
		"password":      c.password,
		"applicationId": appID,
	})
	resp, err := c.http.Post(
		c.base+"/General/LoginPublisherAccountByName",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("dexcom login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dexcom login: status %d", resp.StatusCode)
	}
	var sessionID string
	if err := json.NewDecoder(resp.Body).Decode(&sessionID); err != nil {
		return fmt.Errorf("dexcom login: decode session: %w", err)
	}
	c.sessionID = sessionID
	return nil
}

func (c *Client) FetchLatest(account string) (*types.Reading, error) {
	if c.sessionID == "" {
		if err := c.Login(); err != nil {
			return nil, err
		}
	}
	reading, err := c.fetchLatestRaw(account)
	if errors.Is(err, ErrSessionExpired) {
		if loginErr := c.Login(); loginErr != nil {
			return nil, loginErr
		}
		return c.fetchLatestRaw(account)
	}
	return reading, err
}

type dexcomReading struct {
	WT    string `json:"WT"`
	Value int    `json:"Value"`
	Trend string `json:"Trend"`
}

var dateRe = regexp.MustCompile(`/?Date\((\d+)`)

func (c *Client) fetchLatestRaw(account string) (*types.Reading, error) {
	url := fmt.Sprintf("%s/Publisher/ReadPublisherLatestGlucoseValues?sessionId=%s&minutes=10&maxCount=1",
		c.base, c.sessionID)
	resp, err := c.http.Post(url, "application/json", strings.NewReader(""))
	if err != nil {
		return nil, fmt.Errorf("dexcom fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusInternalServerError {
		c.sessionID = ""
		return nil, ErrSessionExpired
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dexcom fetch: status %d", resp.StatusCode)
	}
	var readings []dexcomReading
	if err := json.NewDecoder(resp.Body).Decode(&readings); err != nil {
		return nil, fmt.Errorf("dexcom fetch: decode: %w", err)
	}
	if len(readings) == 0 {
		return nil, nil
	}
	r := readings[0]
	ms, err := parseWallTime(r.WT)
	if err != nil {
		return nil, fmt.Errorf("dexcom fetch: parse time: %w", err)
	}
	return &types.Reading{
		Account:    account,
		Value:      r.Value,
		Trend:      mapTrend(r.Trend),
		RecordedAt: time.UnixMilli(ms).UTC(),
	}, nil
}

func parseWallTime(wt string) (int64, error) {
	m := dateRe.FindStringSubmatch(wt)
	if len(m) < 2 {
		return 0, fmt.Errorf("unexpected WT format: %s", wt)
	}
	return strconv.ParseInt(m[1], 10, 64)
}

func mapTrend(t string) types.Trend {
	switch t {
	case "DoubleUp":
		return types.TrendDoubleUp
	case "SingleUp":
		return types.TrendSingleUp
	case "FortyFiveUp":
		return types.TrendFortyFiveUp
	case "Flat":
		return types.TrendFlat
	case "FortyFiveDown":
		return types.TrendFortyFiveDown
	case "SingleDown":
		return types.TrendSingleDown
	case "DoubleDown":
		return types.TrendDoubleDown
	case "NotComputable":
		return types.TrendNotComputable
	case "RateOutOfRange":
		return types.TrendRateOutOfRange
	default:
		return types.TrendNone
	}
}
