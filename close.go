package mongo

import (
	"context"
	"errors"
	"time"

	mgo "github.com/globalsign/mgo"
)

// Graceful represents an interface to the shutdown method
type Graceful interface {
	shutdown(ctx context.Context, session *mgo.Session, closedChannel chan bool)
}

type graceful struct{}

func (t graceful) shutdown(ctx context.Context, session *mgo.Session, closedChannel chan bool) {
	session.Close()

	defer func() {
		if x := recover(); x != nil {
			// do nothing ... just handle timing corner case and avoid "panic: send on closed channel"
		}
	}()

	closedChannel <- true
	return
}

var (
	start    Graceful = graceful{}
	timeLeft          = 1000 * time.Millisecond
)

// Close represents mongo session closing within the context deadline
func Close(ctx context.Context, session *mgo.Session) error {
	closedChannel := make(chan bool)
	defer close(closedChannel)

	// Make a copy of timeLeft so that we don't modify the global var
	closeTimeLeft := timeLeft
	if deadline, ok := ctx.Deadline(); ok {
		// Add some time to timeLeft so case where ctx.Done in select
		// statement below gets called before time.After(timeLeft) gets called.
		// This is so the context error is returned over hardcoded error.
		closeTimeLeft = deadline.Sub(time.Now()) + (10 * time.Millisecond)
	}

	go func() {
		start.shutdown(ctx, session, closedChannel)
		return
	}()

	select {
	case <-time.After(closeTimeLeft):
		return errors.New("closing mongo timed out")
	case <-closedChannel:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
