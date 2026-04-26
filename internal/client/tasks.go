package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

type Task struct {
	ID          string          `json:"id"`
	TaskType    string          `json:"task_type"`
	Status      string          `json:"status"`
	Model       string          `json:"model,omitempty"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at,omitempty"`
	CompletedAt string          `json:"completed_at,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       json.RawMessage `json:"error,omitempty"`
	Progress    float64         `json:"progress,omitempty"`
}

type TaskList struct {
	Data []Task `json:"data"`
}

type ListTasksOptions struct {
	TaskType string
	Status   string
	Limit    int
}

func (c *Client) ListTasks(ctx context.Context, opts ListTasksOptions) (*TaskList, error) {
	q := url.Values{}
	if opts.TaskType != "" {
		q.Set("task_type", opts.TaskType)
	}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}
	var out TaskList
	if err := c.get(ctx, "/v1/tasks", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetTask(ctx context.Context, id string) (*Task, error) {
	var out Task
	if err := c.get(ctx, "/v1/tasks/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CancelTask(ctx context.Context, id string) (*Task, error) {
	var out Task
	if err := c.post(ctx, "/v1/tasks/"+url.PathEscape(id)+"/cancel", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WaitForTask polls /v1/tasks/{id} every interval until terminal status or ctx done.
// onUpdate, if non-nil, is called with each fresh Task snapshot.
func (c *Client) WaitForTask(ctx context.Context, id string, interval time.Duration, onUpdate func(*Task)) (*Task, error) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	for {
		t, err := c.GetTask(ctx, id)
		if err != nil {
			return nil, err
		}
		if onUpdate != nil {
			onUpdate(t)
		}
		switch t.Status {
		case "completed", "failed", "cancelled", "canceled":
			return t, nil
		}
		select {
		case <-ctx.Done():
			return t, ctx.Err()
		case <-time.After(interval):
		}
	}
}
