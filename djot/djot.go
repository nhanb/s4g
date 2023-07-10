// Must run djot.StartService() before using it.
package djot

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"os/exec"
)

//go:embed js/djot.js
var djotJs string

//go:embed js/main.js
var mainJs string

var djotFullScript = djotJs + "\n" + mainJs

const delimiter = 0xff

type djotJSProc struct {
	cmd     *exec.Cmd
	writer  *bufio.Writer
	scanner *bufio.Scanner
}

var service djotJSProc

func StartService() {
	cmd := exec.Command("node", "-e", djotFullScript)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	writer := bufio.NewWriter(stdin)
	scanner := bufio.NewScanner(stdout)
	scanner.Split(splitAtDelimiter)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}
	go func() {
		errscanner := bufio.NewScanner(stderr)
		for errscanner.Scan() {
			errtext := errscanner.Text()
			fmt.Printf("XXX %s\n", errtext)
		}
	}()

	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	service = djotJSProc{cmd: cmd, writer: writer, scanner: scanner}
}

func splitAtDelimiter(data []byte, atEOF bool) (advance int, token []byte, err error) {
	idx := bytes.IndexByte(data, delimiter)
	if idx == -1 {
		return 0, nil, nil
	}
	return idx + 1, data[:idx], nil
}

// Not thread-safe.
func ToHtml(input []byte) (result []byte) {
	if _, err := service.writer.Write(input); err != nil {
		panic(err)
	}
	if err := service.writer.WriteByte(delimiter); err != nil {
		panic(err)
	}
	service.writer.Flush()

	if !service.scanner.Scan() {
		panic(fmt.Sprintf(
			"scanner unexpectedly stopped while converting djot to html: %s\n",
			input[:50],
		))
	}
	return service.scanner.Bytes()
}
