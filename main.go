package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	yaml "gopkg.in/yaml.v2"
)

const DEFAULT_VPNC_SCRIPT = "/usr/lib/openconnect-helper/script"

type creds struct {
	Username string `yaml:"username,omitempty"`
	Password string
	TOTP     string `yaml:"totp,omitempty"`
}

func init() {
	log.SetFlags(log.Lshortfile)
	log.SetOutput(os.Stderr)
}

func checkEnv() error {
	for _, env := range []string{
		"PASS_NAME",
		"PROTOCOL",
	} {
		if os.Getenv(env) == "" {
			return fmt.Errorf("environment variable %s not set", env)
		}
	}
	return nil
}

func newCreds(passName, stripDomain string) (*creds, error) {
	cmd := exec.Command("gopass", "show", "-f", passName)
	out, err := cmd.Output()
	if err != nil {
		var b strings.Builder
		fmt.Fprintf(&b, "error running cmd: %v", err)
		switch err := err.(type) {
		case *exec.ExitError:
			fmt.Fprintf(&b, "out: %s; err: %s", out, err.Stderr)
		}
		fmt.Fprint(&b, "\n")
		return nil, fmt.Errorf(b.String())
	}

	split := bytes.Split(out, []byte("---\n"))
	var cred creds
	cred.Password = string(bytes.Split(split[0], []byte("\n"))[0])
	if err := yaml.Unmarshal(split[1], &cred); err != nil {
		log.Printf("error unmarshalling YAML: %v\n", err)
	}

	if stripDomain != "" {
		cred.Username = strings.Split(cred.Username, "@")[0]
	}

	return &cred, nil
}

func getTOTP(passName string) (string, error) {
	cmd := exec.Command("gopass", "otp", passName)
	out, err := cmd.Output()
	if err != nil {
		var b strings.Builder
		fmt.Fprintf(&b, "error running cmd: %v", err)
		switch err := err.(type) {
		case *exec.ExitError:
			fmt.Fprintf(&b, "out: %s; err: %s", out, err.Stderr)
		}
		fmt.Fprint(&b, "\n")
		return "", fmt.Errorf(b.String())
	}

	return string(bytes.Fields(out)[0]), nil
}

func startOpenconnect(cred *creds, authgroup, totp, protocol, iface, script, extraArgs, server string) (*exec.Cmd, error) {
	var b strings.Builder
	b.WriteString(cred.Password)
	b.WriteString("\n")
	if authgroup != "" {
		b.WriteString(authgroup)
		b.WriteString("\n")
	}
	b.WriteString(totp)
	b.WriteString("\n")

	args := []string{
		"--protocol", protocol,
		"--user", cred.Username,
		"--passwd-on-stdin",
		"--interface", iface,
		"--script", script,
	}
	args = append(args, strings.Fields(extraArgs)...)
	args = append(args, server)

	cmd := exec.Command(
		"openconnect",
		args...,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("error creating stdin pipe: %v", err)
	}
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, b.String())
	}()

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("running cmd: %s\n", cmd.Args)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting cmd: %v", err)
	}
	return cmd, nil
}

func main() {
	if err := checkEnv(); err != nil {
		log.Fatal(err)
	}

	if len(os.Args) != 2 && os.Args[1] != "" {
		log.Fatal("no server specified")
	}

	passName := os.Getenv("PASS_NAME")
	cred, err := newCreds(passName, os.Getenv("STRIP_DOMAIN"))
	if err != nil {
		log.Fatal(err)
	}

	var totp string
	otpName := os.Getenv("OTP_NAME")
	switch {
	case otpName != "":
		totp, err = getTOTP(otpName)
	case cred.TOTP != "":
		totp, err = getTOTP(passName)
	}
	if err != nil {
		log.Fatal(err)
	}

	extra_args := os.Getenv("EXTRA_ARGS")
	authgroup := os.Getenv("AUTHGROUP")
	if authgroup != "" {
		extra_args += "--usergroup portal"
	}
	script := os.Getenv("SCRIPT")
	if script == "" {
		script = DEFAULT_VPNC_SCRIPT
	}
	process, err := startOpenconnect(
		cred,
		authgroup,
		totp,
		os.Getenv("PROTOCOL"),
		fmt.Sprintf("tun-%.9s", os.Args[1]),
		script,
		extra_args,
		os.Args[1],
	)
	if err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan struct{}, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Printf("received signal %v", sig)
		process.Process.Signal(sig)
		close(done)
	}()

	process.Wait()
	log.Printf("exiting")
}
