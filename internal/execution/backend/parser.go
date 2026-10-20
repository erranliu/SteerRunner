package backend

import "context"

// ParserTask is only a live resource handle. Durable identity and results stay
// in the original operation and its existing started/final journals.
type ParserTask interface {
	Cancel()
	Done() <-chan struct{}
	Result() (LifecycleReceipt, bool)
}

type ParserOwnerPort interface {
	LifecycleExecutePort
	CancelParser(context.Context, LifecycleRequest) (LifecycleReceipt, error)
}
