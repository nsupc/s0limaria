package ns

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	client             http.Client
	user               string
	requests           []time.Time
	ratelimitLimit     int
	ratelimitRemaining int
	ratelimitResetIn   time.Duration
	maxRequests        int
}

func (c *Client) clearBucket() {
	now := time.Now()

	filtered := []time.Time{}

	for _, instant := range c.requests {
		if now.Sub(instant) <= 30*time.Second {
			filtered = append(filtered, instant)
		}
	}

	c.requests = filtered
}

func (c *Client) acquire() error {
	c.clearBucket()

	if len(c.requests) >= c.maxRequests {
		return errors.New("too many requests")
	}

	if c.ratelimitRemaining <= 1 {
		return errors.New("too many requests")
	}

	now := time.Now()

	c.requests = append(c.requests, now)

	return nil
}

type RecruitmentStatus struct {
	Name       string
	Region     string
	CanRecruit bool
}

func (r *RecruitmentStatus) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "id" {
			r.Name = attr.Value
		}
	}

	type Aux struct {
		Region     string `xml:"region"`
		CanRecruit string `xml:"TGCANRECRUIT"`
	}

	var aux Aux
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}

	r.Region = strings.ToLower(strings.ReplaceAll(aux.Region, " ", "_"))
	r.CanRecruit = aux.CanRecruit == "1"

	return nil
}

func (c *Client) RecruitmentEligible(name string, region string) (RecruitmentStatus, error) {
	nationName := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), " ", "_")
	regionName := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(region)), " ", "_")

	status := RecruitmentStatus{}

	url := fmt.Sprintf("https://www.nationstates.net/cgi-bin/api.cgi?nation=%s&q=region+tgcanrecruit;from=%s", nationName, regionName)

	err := c.acquire()
	if err != nil {
		return status, err
	}

	req, err := http.NewRequest("GET", url, http.NoBody)
	if err != nil {
		return status, err
	}

	req.Header.Add("User-Agent", c.user)

	resp, err := c.client.Do(req)
	if err != nil {
		return status, err
	}
	defer resp.Body.Close()

	limit := resp.Header.Get("ratelimit-limit")
	if limit != "" {
		limit, err := strconv.Atoi(limit)
		if err != nil {
			slog.Warn("failed to convert ratelimit-limit to int", slog.Any("error", err))
		} else {
			c.ratelimitLimit = limit
		}

	}

	remaining := resp.Header.Get("ratelimit-remaining")
	if remaining != "" {
		remaining, err := strconv.Atoi(remaining)
		if err != nil {
			slog.Warn("failed to convert ratelimit-remaining to int", slog.Any("error", err))
		} else {
			c.ratelimitRemaining = remaining
		}
	}

	reset := resp.Header.Get("ratelimit-reset")
	if reset != "" {
		reset, err := strconv.Atoi(reset)
		if err != nil {
			slog.Warn("failed to convert ratelimit-reset to int", slog.Any("error", err))
		} else {
			c.ratelimitResetIn = time.Duration(reset)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return status, err
	}

	err = xml.Unmarshal(body, &status)
	if err != nil {
		return status, err
	}

	return status, nil
}

func New(user string, maxRequests int) *Client {
	client := Client{
		client:             http.Client{},
		user:               user,
		requests:           []time.Time{},
		ratelimitLimit:     50,
		ratelimitRemaining: 50,
		ratelimitResetIn:   30 * time.Second,
		maxRequests:        maxRequests,
	}

	return &client
}
