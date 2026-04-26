package client

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// SSEEvent is one parsed Server-Sent Events frame.
type SSEEvent struct {
	Event string
	Data  []byte
	ID    string
}

// IsDone reports whether this frame is the OpenAI [DONE] sentinel.
func (e SSEEvent) IsDone() bool {
	return bytes.Equal(bytes.TrimSpace(e.Data), []byte("[DONE]"))
}

// streamSSE issues req and forwards parsed events into out. The returned error
// channel emits a single value (nil on clean stream end). It always closes both
// channels before returning.
func (c *Client) streamSSE(ctx context.Context, req *http.Request) (<-chan SSEEvent, <-chan error) {
	req.Header.Set("Accept", "text/event-stream")
	events := make(chan SSEEvent, 16)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		resp, err := c.doRaw(req)
		if err != nil {
			errs <- err
			return
		}
		defer resp.Body.Close()

		if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
			body, _ := io.ReadAll(resp.Body)
			errs <- fmt.Errorf("unexpected content-type %q: %s", resp.Header.Get("Content-Type"), string(body))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

		var current SSEEvent
		dataLines := make([][]byte, 0, 4)

		flush := func() {
			if len(dataLines) == 0 && current.Event == "" && current.ID == "" {
				return
			}
			current.Data = bytes.Join(dataLines, []byte("\n"))
			select {
			case <-ctx.Done():
			case events <- current:
			}
			current = SSEEvent{}
			dataLines = dataLines[:0]
		}

		for scanner.Scan() {
			line := scanner.Bytes()
			// Empty line dispatches the buffered event.
			if len(line) == 0 {
				flush()
				continue
			}
			// Comments start with ':'
			if line[0] == ':' {
				continue
			}
			field, value, _ := bytes.Cut(line, []byte(":"))
			value = bytes.TrimPrefix(value, []byte(" "))
			switch string(field) {
			case "event":
				current.Event = string(value)
			case "data":
				dataLines = append(dataLines, append([]byte(nil), value...))
			case "id":
				current.ID = string(value)
			default:
				// retry / unknown field — ignore.
			}
		}
		flush()

		if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
			errs <- err
		}
	}()

	return events, errs
}
