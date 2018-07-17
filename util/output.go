package iptbutil

import (
	"github.com/ipfs/iptb/testbed/interfaces"
	"io"
)

type Output struct {
	args []string

	exitcode int

	err    error
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func NewOutput(args []string, stdout, stderr io.ReadCloser, exitcode int, cmderr error) (testbedi.TBOutput, error) {

	return &Output{
		args:     args,
		stdout:   stdout,
		stderr:   stderr,
		exitcode: exitcode,
		err:      cmderr,
	}, nil
}

func (o *Output) Args() []string {
	return o.args
}

func (o *Output) Error() error {
	return o.err
}
func (o *Output) ExitCode() int {
	return o.exitcode
}

func (o *Output) Stdout() io.ReadCloser {
	return o.stdout
}

func (o *Output) Stderr() io.ReadCloser {
	return o.stderr
}
