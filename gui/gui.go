package gui

import (
	"bufio"
	"fmt"
	"os/exec"

	"go.imnhan.com/webmaker2000/gui/ipc"
)

func Start(tclPath string) {
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

	go func() {
		errscanner := bufio.NewScanner(stderr)
		for errscanner.Scan() {
			errtext := errscanner.Text()
			fmt.Printf("XXX %s\n", errtext)
		}
	}()

	fmt.Fprintln(stdin, `source -encoding "utf-8" tcl/main.tcl`)
	println("Loaded main tcl script.")

	fmt.Fprintln(stdin, "initialize")

	respond := func(values ...string) {
		ipc.Respond(stdin, values)
	}

	for req := range ipc.Requests(stdout) {
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
