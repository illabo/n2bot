package fatalist

type Fatalist struct {
	logErrChan   chan error
	fatalErrChan chan error
}

func (f *Fatalist) LogError(err error) {
	f.logErrChan <- err
}

func (f *Fatalist) FatalError(err error) {
	f.fatalErrChan <- err
}

func (f *Fatalist) GetLogChan() <-chan error {
	return f.logErrChan
}

func (f *Fatalist) GetFatalChan() <-chan error {
	return f.fatalErrChan
}

// Prevent channels closure in case if listener(s) weren't set in time.
func (f *Fatalist) chanHelper() {
	for {
		select {
		case e := <-f.logErrChan:
			f.logErrChan <- e
		case e := <-f.fatalErrChan:
			f.fatalErrChan <- e
		}
	}
}

func New() *Fatalist {
	f := &Fatalist{
		make(chan error),
		make(chan error),
	}
	go f.chanHelper()
	return f
}
