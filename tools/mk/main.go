package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var targets = []string{
	"linux/amd64",
	"linux/arm64",
	"darwin/amd64",
	"darwin/arm64",
	"windows/amd64",
	"windows/arm64",
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mk help|release|lint|clean")
		os.Exit(2)
	}

	var err error

	switch os.Args[1] {
	case "help":
		err = showHelp(os.Args[2:])
	case "release":
		err = release(os.Args[2:])
	case "lint":
		err = lint()
	case "clean":
		err = clean()
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "mk:", err)
		os.Exit(1)
	}
}

func showHelp(args []string) error {
	fs := flag.NewFlagSet("help", flag.ExitOnError)
	version := fs.String("version", "", "version to print")

	if err := fs.Parse(args); err != nil {
		return err
	}

	binary := "hdf"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	if *version != "" {
		fmt.Printf("hdf %s on %s/%s\n\n", *version, runtime.GOOS, runtime.GOARCH)
	} else {
		fmt.Printf("hdf on %s/%s\n\n", runtime.GOOS, runtime.GOARCH)
	}

	rows := [][2]string{
		{"make build", "compile bin/" + binary},
		{"make run", "run from source"},
		{"make test", "run the tests"},
		{"make vet", "go vet"},
		{"make fmt", "rewrite files with gofmt"},
		{"make lint", "vet plus a gofmt check"},
		{"make check", "lint plus tests"},
		{"make install", "go install into GOBIN"},
		{"make release", "cross-build every target into dist/"},
		{"make clean", "remove bin/ and dist/"},
	}

	for _, row := range rows {
		fmt.Printf("  %-14s %s\n", row[0], row[1])
	}

	fmt.Println()
	fmt.Println("cross-build with: make build GOOS=windows GOARCH=amd64")

	return nil
}

func release(args []string) error {
	fs := flag.NewFlagSet("release", flag.ExitOnError)
	version := fs.String("version", "dev", "version stamped into the binary")
	repo := fs.String("repo", "", "repository stamped into the binary")
	dir := fs.String("out", "dist", "directory to write binaries to")

	if err := fs.Parse(args); err != nil {
		return err
	}

	ldflags := "-s -w -X main.version=" + *version
	if *repo != "" {
		ldflags += " -X main.repo=" + *repo
	}

	if err := os.RemoveAll(*dir); err != nil {
		return err
	}

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return err
	}

	names := make([]string, 0, len(targets))

	for _, target := range targets {
		goos, goarch, _ := strings.Cut(target, "/")
		name := fmt.Sprintf("hdf_%s_%s_%s", *version, goos, goarch)

		if goos == "windows" {
			name += ".exe"
		}

		fmt.Printf("%-46s", name)

		cmd := exec.Command(goCommand(), "build",
			"-trimpath",
			"-buildvcs=false",
			"-ldflags", ldflags,
			"-o", filepath.Join(*dir, name),
			".",
		)
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Println("failed")
			return err
		}

		fmt.Println("ok")
		names = append(names, name)
	}

	lines, err := checksums(*dir, names)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(*dir, "sha256.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return err
	}

	for _, line := range lines {
		fmt.Println(line)
	}

	return nil
}

func checksums(dir string, names []string) ([]string, error) {
	sort.Strings(names)
	lines := make([]string, 0, len(names))

	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}

		sum := sha256.Sum256(data)
		lines = append(lines, hex.EncodeToString(sum[:])+"  "+name)
	}

	return lines, nil
}

func lint() error {
	out, err := exec.Command(gofmt(), "-l", ".").CombinedOutput()
	if err != nil {
		return fmt.Errorf("gofmt: %w", err)
	}

	if unformatted := strings.TrimSpace(string(out)); unformatted != "" {
		return fmt.Errorf("gofmt needed:\n%s", unformatted)
	}

	cmd := exec.Command(goCommand(), "vet", "./...")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func goCommand() string {
	if name := os.Getenv("GO"); name != "" {
		return name
	}

	return "go"
}

func gofmt() string {
	if path, err := exec.LookPath("gofmt"); err == nil {
		return path
	}

	name := "gofmt"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	return filepath.Join(runtime.GOROOT(), "bin", name)
}

func clean() error {
	for _, dir := range []string{"bin", "dist"} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}

	return nil
}
