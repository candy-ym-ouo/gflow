package action

import (
	"bytes"
	"context"
	"encoding/json"
	"example.com/gflow/internal/model"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Executor struct {
	client    *http.Client
	functions map[string]Function
	seen      map[string]map[string]any
}
type Function func(context.Context, map[string]any) (map[string]any, error)

func New() *Executor {
	e := &Executor{client: &http.Client{Timeout: 10 * time.Second}, functions: map[string]Function{}, seen: map[string]map[string]any{}}
	e.Register("echo", func(_ context.Context, input map[string]any) (map[string]any, error) { return input, nil })
	e.Register("set", setFunction)
	return e
}
func (e *Executor) Register(name string, fn Function) { e.functions[name] = fn }
func (e *Executor) Run(ctx context.Context, n model.Node, input map[string]any) (map[string]any, error) {
	k := fmt.Sprint(n.Config["kind"])
	key := fmt.Sprint(n.Config["idempotencyKey"])
	if key != "" {
		if value, ok := e.seen[key]; ok {
			return clone(value), nil
		}
	}
	var output map[string]any
	var err error
	switch k {
	case "wait":
		d, parseErr := time.ParseDuration(fmt.Sprint(n.Config["duration"]))
		if parseErr != nil {
			return nil, parseErr
		}
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		output = input
	case "notify":
		output = map[string]any{"notified": true, "channel": n.Config["channel"], "input": input}
	case "function":
		name := fmt.Sprint(n.Config["name"])
		fn, ok := e.functions[name]
		if !ok {
			return nil, fmt.Errorf("unknown function %s", name)
		}
		output, err = fn(ctx, input)
	case "http":
		output, err = e.runHTTP(ctx, n, input)
	case "http_batch":
		output, err = e.runHTTPBatch(ctx, n, input)
	case "fail":
		return nil, fmt.Errorf("action failed")
	default:
		return nil, fmt.Errorf("unsupported action kind %s", k)
	}
	if err != nil {
		return nil, err
	}
	if output == nil {
		output = map[string]any{}
	}
	if key != "" {
		e.seen[key] = clone(output)
	}
	return output, nil
}

func (e *Executor) runHTTPBatch(ctx context.Context, n model.Node, input map[string]any) (map[string]any, error) {
	urls, _ := n.Config["urls"].([]any)
	results := make([]any, 0, len(urls))
	for _, raw := range urls {
		result, err := e.doBatchRequest(ctx, fmt.Sprint(raw), input)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return map[string]any{"responses": results}, nil
}

// doBatchRequest performs a single batch HTTP request and closes the response
// body as soon as it has been read (or on any error). Closing per request —
// rather than deferring to the end of the whole batch — keeps the number of
// open connections and file descriptors bounded, so large batches don't run
// into "too many open files" and mid-batch failures don't leak responses.
func (e *Executor) doBatchRequest(ctx context.Context, url string, input map[string]any) (map[string]any, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	response, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": response.StatusCode, "body": string(data)}, nil
}

func (e *Executor) runHTTP(ctx context.Context, n model.Node, input map[string]any) (map[string]any, error) {
	url := fmt.Sprint(n.Config["url"])
	if url == "" {
		return nil, fmt.Errorf("http action requires url")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	method := strings.ToUpper(fmt.Sprint(n.Config["method"]))
	if method == "" {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("http action returned %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	out := map[string]any{"status": response.StatusCode}
	if len(data) > 0 {
		var decoded any
		if json.Unmarshal(data, &decoded) == nil {
			out["body"] = decoded
		} else {
			out["body"] = string(data)
		}
	}
	return out, nil
}
func setFunction(_ context.Context, input map[string]any) (map[string]any, error) {
	return clone(input), nil
}
func clone(input map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range input {
		out[key] = value
	}
	return out
}
