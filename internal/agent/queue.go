package agent

import (
	"context"
	"sync"
)

type llmJob struct {
	ctx  context.Context
	req  ChatRequest
	resp chan llmResult
}

type llmResult struct {
	out *ChatResponse
	err error
}

// Queue serializes LLM calls through a single worker (minimal load).
type Queue struct {
	client LLMClient
	ch     chan llmJob
	wg     sync.WaitGroup
}

func NewQueue(client LLMClient, buffer int) *Queue {
	if buffer < 1 {
		buffer = 16
	}
	q := &Queue{
		client: client,
		ch:     make(chan llmJob, buffer),
	}
	q.wg.Add(1)
	go q.worker()
	return q
}

func (q *Queue) worker() {
	defer q.wg.Done()
	for job := range q.ch {
		out, err := q.client.Chat(job.ctx, job.req)
		job.resp <- llmResult{out: out, err: err}
	}
}

func (q *Queue) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	respCh := make(chan llmResult, 1)
	select {
	case q.ch <- llmJob{ctx: ctx, req: req, resp: respCh}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case r := <-respCh:
		return r.out, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (q *Queue) Close() {
	close(q.ch)
	q.wg.Wait()
}
