package statuszlib

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

var (
	requestCount int32
	sucessCount  int32
	errorCount   int32
)

func HandleStatusz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "requests: %d\nsuccesses: %d\nerrors: %d\n", requestCount, sucessCount, errorCount)
}

func RecordRequest() int32 {
	return atomic.AddInt32(&requestCount, 1)
}

func RecordSuccess() int32 {
	return atomic.AddInt32(&sucessCount, 1)
}

func RecordError() int32 {
	return atomic.AddInt32(&errorCount, 1)
}
