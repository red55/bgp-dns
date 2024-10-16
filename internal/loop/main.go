package loop

import (
	//"context"
	//"time"
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

func (l *Loop) Operation(f func () error) (e error) {
	ec := make(chan error)
	defer func() { e = <-ec }()
	l.Chan() <- &loopOp{
		f:     f,
		errCh: ec,
	}
	return
}

func (l *Loop) NoOp(o *loopOp) {
	o.errCh <- nil
}
func (l *Loop) HandleOp(o *loopOp) {
	o.errCh <- o.f()
}
