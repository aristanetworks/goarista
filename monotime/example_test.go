package monotime_test

import (
	"fmt"
	"time"

	"github.com/aristanetworks/goarista/monotime"
)

func Example() {
	start := monotime.Now()
	time.Sleep(1 * time.Nanosecond)
	fmt.Println(monotime.Since(start))
}
