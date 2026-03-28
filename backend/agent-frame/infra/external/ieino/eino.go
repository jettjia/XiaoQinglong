package ieino

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jettjia/xiaoqinglong/agent-frame/config"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type AskRequest struct {
	Question         string          `json:"question"`
	TimeoutMS        int             `json:"timeout_ms,omitempty"`
	AgentType        string          `json:"agent_type,omitempty"`
	Orchestration    json.RawMessage `json:"orchestration,omitempty"`
	Messages         []Message       `json:"messages,omitempty"`
	RetrievalResults []RetrievalResult `json:"retrieval_results,omitempty"`
}

type RetrievalResult struct {
	Content string  `json:"content"`
	Score   float64 `json:"score"`
	Source  string  `json:"source,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AskResponse struct {
	Question           string `json:"question"`
	Answer             string `json:"answer"`
	RetrievedDocsCount int    `json:"retrieved_docs_count"`
	FinishedAt         string `json:"finished_at"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type sseDelta struct {
	Text string `json:"text"`
}

type sseDone struct {
	Question    string `json:"question"`
	Content     string `json:"content"`
	FinishedAt  string `json:"finished_at"`
	Intent      string `json:"intent,omitempty"`
	Agent       string `json:"agent,omitempty"`
	StartedAt   string `json:"started_at,omitempty"`
	StreamProto string `json:"stream_protocol,omitempty"`
}

func NewClientFromConfig() *Client {
	cfg := config.NewConfig()
	baseURL := "http://127.0.0.1:8090"
	if cfg != nil && cfg.Third.Extra != nil {
		if v, ok := cfg.Third.Extra["eino_base_url"]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				baseURL = strings.TrimSpace(s)
			}
		}
	}
	return NewClient(baseURL, 0)
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	base := strings.TrimSpace(baseURL)
	base = strings.TrimRight(base, "/")
	return &Client{
		baseURL: base,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Ask(ctx context.Context, req AskRequest) (*AskResponse, error) {
	if strings.TrimSpace(req.Question) == "" {
		return nil, fmt.Errorf("question is required")
	}

	resp, err := c.AskStream(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	answer, finishedAt, err := parseSSEToAnswer(resp.Body)
	if err != nil {
		return nil, err
	}

	return &AskResponse{
		Question:           strings.TrimSpace(req.Question),
		Answer:             answer,
		RetrievedDocsCount: 0,
		FinishedAt:         finishedAt,
	}, nil
}

func (c *Client) AskStream(ctx context.Context, req AskRequest) (*http.Response, error) {
	if strings.TrimSpace(req.Question) == "" {
		return nil, fmt.Errorf("question is required")
	}

	endpoint := c.baseURL + "/v1/agent/ask/stream"
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("go-eino http %d", resp.StatusCode)
		}
		var er ErrorResponse
		if err := json.Unmarshal(respBody, &er); err == nil && strings.TrimSpace(er.Error) != "" {
			return nil, fmt.Errorf("go-eino error: %s", er.Error)
		}
		return nil, fmt.Errorf("go-eino http %d: %s", resp.StatusCode, string(respBody))
	}

	return resp, nil
}

func (c *Client) Registry(ctx context.Context) (map[string]any, error) {
	endpoint := c.baseURL + "/v1/agent/registry"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var er ErrorResponse
		if err := json.Unmarshal(body, &er); err == nil && strings.TrimSpace(er.Error) != "" {
			return nil, fmt.Errorf("go-eino error: %s", er.Error)
		}
		return nil, fmt.Errorf("go-eino http %d: %s", resp.StatusCode, string(body))
	}

	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseSSEToAnswer(r io.Reader) (answer string, finishedAt string, err error) {
	br := bufio.NewReader(r)
	var currentEvent string
	var currentData strings.Builder
	var out strings.Builder
	var done sseDone

	flush := func() error {
		ev := strings.TrimSpace(currentEvent)
		data := strings.TrimSpace(currentData.String())
		currentEvent = ""
		currentData.Reset()

		if ev == "" && data == "" {
			return nil
		}

		switch ev {
		case "delta":
			var d sseDelta
			if err := json.Unmarshal([]byte(data), &d); err == nil && strings.TrimSpace(d.Text) != "" {
				out.WriteString(d.Text)
			}
		case "done":
			_ = json.Unmarshal([]byte(data), &done)
		}
		return nil
	}

	for {
		line, readErr := br.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return "", "", readErr
		}
		l := strings.TrimRight(line, "\r\n")

		if l == "" {
			if err := flush(); err != nil {
				return "", "", err
			}
		} else if strings.HasPrefix(l, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(l, "event:"))
		} else if strings.HasPrefix(l, "data:") {
			if currentData.Len() > 0 {
				currentData.WriteByte('\n')
			}
			currentData.WriteString(strings.TrimSpace(strings.TrimPrefix(l, "data:")))
		}

		if readErr == io.EOF {
			break
		}
	}

	_ = flush()

	if strings.TrimSpace(done.Content) != "" {
		return done.Content, done.FinishedAt, nil
	}

	return out.String(), done.FinishedAt, nil
}
