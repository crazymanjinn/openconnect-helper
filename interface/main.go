package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
}

func printUsageAndQuit() {
	log.Fatalf("Usage: %s {start|stop} {interface} [user]\n", os.Args[0])
}

func newArgs(action, iface string, extra ...string) []string {
	args := []string{"tuntap"}
	args = append(args,
		action,
		strings.Replace(fmt.Sprintf("%.13s", iface), ".", "-", -1),
		"mode",
		"tun",
	)
	if len(extra) > 0 {
		args = append(args, extra...)
	}
	return args
}

func main() {
	if len(os.Args) < 2 {
		printUsageAndQuit()
	}

	var args []string
	switch os.Args[1] {
	case "start":
		if len(os.Args) < 4 {
			log.Println("not enough arguments; expecting interface and user")
			printUsageAndQuit()
		}
		args = newArgs("add", os.Args[2], "user", os.Args[3])
	case "stop":
		if len(os.Args) < 3 {
			log.Println("not enough arguments; expecting interface")
			printUsageAndQuit()
		}
		args = newArgs("del", os.Args[2])
	default:
		printUsageAndQuit()
	}

	cmd := exec.Command("ip", args...)
	log.Printf("running cmd: %s\n", cmd.Args)
	if out, err := cmd.Output(); err != nil {
		log.Printf("error running cmd: %s\n", err)
		switch err := err.(type) {
		case *exec.ExitError:
			log.Printf("stdout: %s, stderr: %s\n", out, err.Stderr)
		}
		os.Exit(1)
	}
}
