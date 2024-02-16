package memoize

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/xid"
)

type TestSuite struct {
	suite.Suite
	retVal    string
	retErr    error
	wrapped   Func[string]
	callCount int64
}

// In order for 'go test' to run this suite, we need to create a normal test function and pass our suite to suite.Run
func TestRunTestSuite(t *testing.T) {
	suite.Run(t, new(TestSuite))
}

func TestCanceledOutside(t *testing.T) {
	wrapped, cancel := Wrap(func(ctx context.Context) (string, error) {
		sleepCtx(ctx, time.Hour)
		return "never", nil
	}, time.Hour)

	done := make(chan bool)
	go func() {
		_, err := wrapped(context.Background())
		require.ErrorIs(t, err, context.Canceled)
		done <- true
	}()

	time.Sleep(3 * time.Millisecond)
	cancel()
	<-done
}

func TestCanceledCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	wrapped, _ := Wrap(func(ctx context.Context) (string, error) {
		sleepCtx(ctx, time.Hour)
		return "never", nil
	}, time.Hour)

	done := make(chan bool)
	go func() {
		_, err := wrapped(ctx)
		require.ErrorIs(t, err, context.Canceled)
		done <- true
	}()

	time.Sleep(3 * time.Millisecond)
	cancel()
	<-done
}

func (s *TestSuite) TestBasic() {
	s.initMemoizeFunc(time.Millisecond*10, func() {})
	s.testConcurrentCall()
}

func (s *TestSuite) TestTtl() {
	s.initMemoizeFunc(time.Millisecond*10, func() {})
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 10)
		s.testConcurrentCall()

		time.Sleep(time.Millisecond)
		n := int(s.callCount)
		s.concurrentCall()
		s.Require().Equal(n, int(s.callCount))

		time.Sleep(time.Millisecond * 10)
		s.testConcurrentCall()
	}
}

func (s *TestSuite) SetupTest() {
	s.retVal = xid.New().String()
	s.retErr = errors.New(xid.New().String())
}

func (s *TestSuite) initMemoizeFunc(ttl time.Duration, f func()) {
	s.wrapped, _ = Wrap(func(ctx context.Context) (string, error) {
		f()
		atomic.AddInt64(&s.callCount, 1)
		return s.retVal, s.retErr
	}, ttl)
}

func (s *TestSuite) call() {
	v, err := s.wrapped(context.Background())
	s.Require().EqualValues(v, s.retVal)
	s.Require().EqualValues(err, s.retErr)
}

func (s *TestSuite) concurrentCall() {
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			s.call()
			wg.Done()
		}()
	}
	wg.Wait()
}

func (s *TestSuite) testConcurrentCall() {
	callCount := int(s.callCount)
	s.concurrentCall()
	s.Require().Equal(callCount+1, int(s.callCount))
}

func sleepCtx(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
	case <-timer.C:
	}
}
