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

func New() Fatalist {
	return Fatalist{
		make(chan error),
		make(chan error),
	}
}
