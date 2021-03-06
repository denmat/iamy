package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/99designs/iamy/Godeps/_workspace/src/gopkg.in/alecthomas/kingpin.v2"
)

var (
	Version    string = "dev"
	defaultDir string
)

type logWriter struct{ *log.Logger }

func (w logWriter) Write(b []byte) (int, error) {
	w.Printf("%s", b)
	return len(b), nil
}

type Ui struct {
	*log.Logger
	Error, Debug *log.Logger
	Exit         func(code int)
}

func main() {
	var (
		debug     = kingpin.Flag("debug", "Show debugging output").Bool()
		pull      = kingpin.Command("pull", "Syncs IAM users, groups and policies from the active AWS account to files")
		pullDir   = pull.Flag("dir", "The directory to dump yaml files to").Default(defaultDir).Short('d').String()
		canDelete = pull.Flag("delete", "Delete extraneous files from destination dir").Bool()
		push      = kingpin.Command("push", "Syncs IAM users, groups and policies from files to the active AWS account")
		pushDir   = push.Flag("dir", "The directoy to load yaml files from").Default(defaultDir).Short('d').ExistingDir()
	)

	kingpin.Version(Version)
	kingpin.CommandLine.Help =
		`Read and write AWS IAM users, policies, groups and roles from YAML files.`

	ui := Ui{
		Logger: log.New(os.Stdout, "", 0),
		Error:  log.New(os.Stderr, "", 0),
		Debug:  log.New(ioutil.Discard, "", 0),
		Exit:   os.Exit,
	}

	cmd := kingpin.Parse()

	if *debug {
		ui.Debug = log.New(os.Stderr, "DEBUG ", log.LstdFlags)
		log.SetFlags(0)
		log.SetOutput(&logWriter{ui.Debug})
	} else {
		log.SetOutput(ioutil.Discard)
	}

	switch cmd {
	case push.FullCommand():
		PushCommand(ui, PushCommandInput{
			Dir: *pushDir,
		})

	case pull.FullCommand():
		PullCommand(ui, PullCommandInput{
			Dir:       *pullDir,
			CanDelete: *canDelete,
		})
	}
}

func init() {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		panic(err)
	}
	defaultDir = filepath.Clean(dir)
}
