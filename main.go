package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/melbahja/goph"

	"github.com/neovim/go-client/nvim"
	"github.com/neovim/go-client/nvim/plugin"
)

const (
	NEOVIM_REMOTE_PATTERN = `remote://*`
)

var (
	Session *goph.Client
)

type NeovimRemote struct {
	p *plugin.Plugin
}

type AutocmdArgs struct {
	Buffer     nvim.Buffer `eval:"bufnr()"`
	BufferName string      `eval:"bufname()"`
}

func NewGophSession() (*goph.Client, error) {
	auth, err := goph.Key("", "") // TODO
	if err != nil {
		return nil, err
	}
	client, err := goph.NewUnknown("root", "", auth) // TODO
	if err != nil {
		return nil, err
	}
	return client, err
}

func DownloadSshFile(client *goph.Client, filename string) ([][]byte, error) {
	sftp, err := client.NewSftp()
	if err != nil {
		return nil, err
	}
	defer sftp.Close()

	// stat, err := sftp.Stat("/root/main.go")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	remote, err := sftp.Open(filename)
	if err != nil {
		return nil, err
	}
	defer remote.Close()

	result, err := io.ReadAll(remote)
	if err != nil {
		return nil, err
	}

	return bytes.Split(result, []byte("\n")), nil
}

func UploadSshFile(client *goph.Client, filename string, lines [][]byte) error {
	sftp, err := client.NewSftp()
	if err != nil {
		return err
	}
	defer sftp.Close()

	// stat, err := sftp.Stat(filename)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	remote, err := sftp.Create(filename)
	if err != nil {
		return err
	}
	defer remote.Close()

	remote.Write(bytes.Join(lines, []byte("\n")))
	return nil
}

func (this *NeovimRemote) HandleBufWriteCmd(args *AutocmdArgs) {
	log.Println("New buf write event:", args)
	lines, err := this.p.Nvim.BufferLines(args.Buffer, 0, -1, true)
	if err != nil {
		log.Println("Error reading buffer lines:", err) // TODO: send error to nvim
		return
	}
	err = UploadSshFile(Session, "/root/main.go", lines)
	if err != nil {
		log.Println("Error writing lines to remote:", err) // TODO: send error to nvim
		return
	}
	err = this.p.Nvim.SetBufferOption(args.Buffer, "modified", false)
	if err != nil {
		log.Println("SetBufferOption error:", err) // TODO: send error to nvim
		return
	}
}

func (this *NeovimRemote) HandleBufReadCmd(args *AutocmdArgs) {
	log.Println("New buf read event:", args)
	err := this.p.Nvim.SetBufferOption(
		args.Buffer,
		"filetype",
		filepath.Ext(args.BufferName)[1:],
	)
	if err != nil {
		log.Println("SetBufferOption error:", err) // TODO: send error to nvim
		return
	}
	content, err := DownloadSshFile(Session, "/root/main.go")
	if err != nil {
		log.Println("Downloadfile error:", err) // TODO: send error to nvim
		return
	}
	err = this.p.Nvim.SetBufferLines(args.Buffer, 0, -1, false, content)
	if err != nil {
		log.Println("SetBufferLines error:", err) // TODO: send error to nvim
		return
	}
	err = this.p.Nvim.SetBufferOption(args.Buffer, "modified", false)
	if err != nil {
		log.Println("SetBufferOption error:", err) // TODO: send error to nvim
		return
	}
}

func main() {
	logFile, err := os.Create("neovim-remote.log") // TODO: should be able to set that in config
	if err != nil {
		panic(err) // TODO
	}
	log.SetOutput(logFile)

	Session, err = NewGophSession()
	if err != nil {
		panic(err)
	}
	defer Session.Close()

	plugin.Main(func(p *plugin.Plugin) error {
		remote := NeovimRemote{p: p}
		// TODO: add vim autocmd group
		p.HandleAutocmd(
			&plugin.AutocmdOptions{Event: "BufReadCmd", Pattern: NEOVIM_REMOTE_PATTERN, Eval: "*"},
			remote.HandleBufReadCmd,
		)
		p.HandleAutocmd(
			&plugin.AutocmdOptions{Event: "FileReadCmd", Pattern: NEOVIM_REMOTE_PATTERN, Eval: "*"},
			remote.HandleBufReadCmd,
		)
		p.HandleAutocmd(
			&plugin.AutocmdOptions{Event: "BufWriteCmd", Pattern: NEOVIM_REMOTE_PATTERN, Eval: "*"},
			remote.HandleBufWriteCmd,
		)
		return nil
	})
}
