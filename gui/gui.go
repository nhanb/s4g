package gui

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"go.imnhan.com/webmaker2000/gui/ipc"
)

//go:embed tcl/main.tcl
var tclMain []byte

//go:embed tcl/choose-task.tcl
var tclChooseTask []byte

func Main(tclPath string) {
	interp := newInterp(tclPath)

	go func() {
		errscanner := bufio.NewScanner(interp.stderr)
		for errscanner.Scan() {
			errtext := errscanner.Text()
			fmt.Printf("XXX %s\n", errtext)
		}
	}()

	_, err := interp.stdin.Write(tclMain)
	if err != nil {
		panic(err)
	}
	println("Loaded main tcl script.")

	fmt.Fprintln(interp.stdin, "initialize")

	respond := func(values ...string) {
		ipc.Respond(interp.stdin, values)
	}

	for req := range ipc.Requests(interp.stdout) {
		switch req.Method {

		case "forcefocus":
			//err := forceFocus(req.Args[0])
			//if err != nil {
			//fmt.Printf("forcefocus: %s\n", err)
			//}
			respond("ok")
		}

	}

	println("Tcl process terminated.")
}

type Task string

const (
	TaskOpen   Task = "open"
	TaskCreate Task = "create"
)

func ChooseTask(tclPath string) (task Task, path string, ok bool) {
	interp := newInterp(tclPath)
	interp.stdin.Write(tclChooseTask)
	interp.stdin.Close()
	resp, err := io.ReadAll(interp.stdout)
	if err != nil {
		panic(err)
	}

	action, path, found := strings.Cut(strings.TrimSpace(string(resp)), " ")
	if !found {
		fmt.Println("No task chosen")
		return "", "", false
	}

	switch action {
	case string(TaskOpen):
		return TaskOpen, filepath.Dir(path), true
	case string(TaskCreate):
		return TaskCreate, path, true
	default:
		fmt.Printf("Unexpected tclChooseTask output: %s\n", string(resp))
		return "", "", false
	}
}

type tclInterp struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func newInterp(tclPath string) *tclInterp {
	cmd := exec.Command(tclPath, "-encoding", "utf-8")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	return &tclInterp{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}
}
