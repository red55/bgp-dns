package loop

import (
	//"context"
	//"time"
//	"github.com/red55/bgp-dns/internal/log"
)

type loopOp struct {
	f 	func () error
	errCh chan error
}

type Loop struct {
	opCh	chan *loopOp
}

func NewLoop(bufSize int) Loop {
	return Loop{
		opCh: make(chan *loopOp, bufSize),
	}
}
func (l *Loop) Chan() chan *loopOp{
	return l.opCh
}

func (l *Loop) Operation(f func () error, ret bool) (e error) {
	//log.L().Trace().Caller(1).Msg("-> loop.Operation")
	var ec chan error
	if ret {
		ec = make(chan error)
	}
	defer func() {
		if nil != ec {
			//log.L().Trace().Caller(2).Msg("-- loop.Read return")
			e = <-ec
		}

		//log.L().Trace().Caller(2).Msg("<- loop.Operation")
	}()
	l.Chan() <- &loopOp{
		f:     f,
		errCh: ec,
	}
	return
}
func (l *Loop) NoErr(o *loopOp) {
	if nil != o.errCh {
		o.errCh <- nil
	}

	_ = o.f()
}

func (l *Loop) NoOp(o *loopOp) {
	if nil != o.errCh {
		o.errCh <- nil
	}
}
func (l *Loop) HandleOp(o *loopOp) {
	if nil != o.errCh {
		o.errCh <- o.f()
	} else {
		_ = o.f()
	}
}
