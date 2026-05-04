package services

type serviceGreeter struct{}

func newServiceGreeter() serviceGreeter {
	return serviceGreeter{}
}

func (serviceGreeter) Greet(name string) string {
	return "hi " + name
}
