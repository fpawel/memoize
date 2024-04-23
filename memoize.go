package memoize

import (
	"context"
	"sync"
	"time"
)

type (
	// Func is the type for functions that can be memoized.
	Func[T any] func(ctx context.Context) (T, error)
)

// Memoize returns a callback to the provided expensive function `f`, caching the result of the call
// and suppressing duplicate calls for the specified time `ttl`.
// When calling the callback resulting from calling Memoize `Wf` for the first time,
// the underlying function `f` will be called. The result of the call `R` will be saved, remembered
// and returned to any call of the callback `Wf` during the `ttl` time.
// After that, the value `R` (the result of the previous call of `f`) is considered expired and
// `f` will be called again with the same guarantees when calling `Wf`.
// In other words, the state of the callback `Wf` after `ttl` time after calling `f`
// goes into the initial state, which was immediately after the return of `Wf` by the Memoize function call.
// The callback resulting from calling Memoize is thread-safe and can be called simultaneously
// from multiple goroutines without violating the invariant or data racing.
//
// Example:
//
//	fun := func (context.Context) (string, error) {
//		fmt.Println("some expensive and long computation")
//		return "result", nil
//	}
//
//	memoized, _ := Memoize(fun, 2*time.Second)
//
//	go memoized(ctx) // ctx := context.Background()
//	go memoized(ctx)
//	go memoized(ctx)
//
//	time.Sleep(time.Second)   // `fun` was called once, printed "some expensive and long computation"
//	fmt.Println(wrapped(ctx)) //  `fun` was not called here, printed "result <nil>"
//
//	time.Sleep(2 * time.Second) // pause while the result of the previous call is relevant
//
//	go memoized(ctx)
//	go memoized(ctx)
//	go memoized(ctx)
//
//	time.Sleep(time.Second)   // `fun` was called exactly one more time, printed "some expensive and long computation"
//	fmt.Println(memoized(ctx)) //  `fun` was not called here, printed "result <nil>"
func Memoize[T any](f Func[T], ttl time.Duration) (Func[T], context.CancelFunc) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	m := &mem[T]{
		ttl: ttl,
		res: result[T]{},
		fun: f,
		ctx: ctx,
	}
	return m.getOrCall, cancelCtx
}

type (
	result[T any] struct {
		val T
		err error
	}
	mem[T any] struct {
		ttl   time.Duration
		res   result[T]
		time  time.Time
		mu    sync.Mutex
		tasks []chan result[T]
		fun   Func[T]
		ctx   context.Context
	}
)

func (p *mem[T]) getOrCall(ctx context.Context) (T, error) {
	p.mu.Lock()
	res, tm := p.res, p.time

	if !tm.IsZero() && (p.ttl == 0 || time.Since(tm) < p.ttl) {
		p.mu.Unlock()
		return res.val, res.err
	}

	ch := make(chan result[T])
	p.tasks = append(p.tasks, ch)
	if len(p.tasks) == 1 {
		go func() {
			v, err := p.fun(p.ctx)
			res := result[T]{v, err}

			p.mu.Lock()
			tasks := p.tasks
			p.res, p.tasks, p.time = res, nil, time.Now()
			p.mu.Unlock()

			for _, task := range tasks {
				task <- res
			}
		}()
	}
	p.mu.Unlock()

	select {
	case <-p.ctx.Done():
		return res.val, p.ctx.Err()
	case <-ctx.Done():
		return res.val, ctx.Err()
	case res = <-ch:
		return res.val, res.err
	}
}
