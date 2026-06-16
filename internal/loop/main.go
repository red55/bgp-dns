package loop

import (
	"github.com/red55/bgp-dns/internal/log"
)

type loopOp struct {
	f     func() error
	errCh chan error
}

type Loop struct {
	opCh chan *loopOp
	l    *log.Log
}

func NewLoop(bufSize int) Loop {
	l := log.NewLog(log.L(), "loop")
	return Loop{
		opCh: make(chan *loopOp, bufSize), //, bufSize
		l:    &l,
	}
}
func (l *Loop) ChanOp() chan *loopOp {
	return l.opCh
}

func (l *Loop) Operation(f func() error, ret bool) (e error) {
	var ec chan error

	if ret {
		ec = make(chan error)
		defer close(ec)
	}

	defer func() {
		if nil != ec {
			e = <-ec
		}
	}()

	l.ChanOp() <- &loopOp{
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
	if nil == o.errCh {
		_ = o.f()
	} else {
		o.errCh <- o.f()
	}
}
