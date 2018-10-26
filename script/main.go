package main

import (
	"log"
	"os"
	"os/exec"
	"path"
	"syscall"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
}

func getScript(script string) string {
	base := "/usr/lib/openconnect-vpnc-scripts"
	var validScripts map[string]string
	validScripts = make(map[string]string)
	for _, s := range []string{
		"vpnc-script",
		"vpnc-script-ptrtd",
		"vpnc-script-sshd",
	} {
		validScripts[s] = path.Join(base, s)
	}
	if v, ok := validScripts[script]; ok {
		return v
	}
	return validScripts["vpnc-script"]
}

func main() {
	var input string
	if len(os.Args) > 2 {
		input = os.Args[1]
	}
	cmd := exec.Command(getScript(input))
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: 0, Gid: 0}
	log.Printf("running cmd: %s\n", cmd.Args)
	if out, err := cmd.Output(); err != nil {
		log.Printf("error running cmd: %s\n", err)
		switch err := err.(type) {
		case *exec.ExitError:
			log.Printf("stdout: %s; stderr: %s\n", out, err.Stderr)
		}
		os.Exit(1)
	}
}
