# memoize

Provides a duplicate function call suppression and caching mechanism. Dead-simple. Safe for concurrent use.

```go
package main 

import (
    "context"
    "fmt"
	"time"

	"github.com/fpawel/memoize"
)

func fun(context.Context) (string, error) {
	fmt.Println("some expensive and long computation")
	return "result", nil
}

func main() {
	memoized, _ := memoize.Memoize(fun, 2*time.Second)
	ctx := context.Background()
	go memoized(ctx)
	go memoized(ctx)
	go memoized(ctx)
	time.Sleep(time.Second)   // `fun` was called once, "some expensive and long computation"
	fmt.Println(memoized(ctx)) //  `fun` was not called here, "result <nil>"

	time.Sleep(2 * time.Second) // pause while the result of the previous call is relevant

	go memoized(ctx)
	go memoized(ctx)
	go memoized(ctx)
	time.Sleep(time.Second)   // `fun` was called exactly one more time, "some expensive and long computation"
	fmt.Println(memoized(ctx)) //  `fun` was not called here, "result <nil>"
}

```
