package pluginlocalfilecoin

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"

	"github.com/filecoin-project/go-filecoin/tools/iptb-plugins/filecoin"
)

func (l *Localfilecoin) isAlive() (bool, error) {
	pid, err := l.getPID()
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, nil
	}

	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (l *Localfilecoin) getPID() (int, error) {
	b, err := ioutil.ReadFile(filepath.Join(l.dir, "daemon.pid"))
	if err != nil {
		return -1, err
	}

	return strconv.Atoi(string(b))
}

func (l *Localfilecoin) env() ([]string, error) {
	envs := os.Environ()

	currPath := os.Getenv("PATH")
	pathList := filepath.SplitList(currPath)
	pathList = append([]string{filepath.Dir(l.binPath)}, pathList...)
	newPath := strings.Join(pathList, string(filepath.ListSeparator))
	envs = filecoin.UpdateOrAppendEnv(envs, "FIL_PATH", l.dir)
	envs = filecoin.UpdateOrAppendEnv(envs, "GO_FILECOIN_LOG_LEVEL", l.logLevel)
	envs = filecoin.UpdateOrAppendEnv(envs, "GO_FILECOIN_LOG_JSON", l.logJSON)
	envs = filecoin.UpdateOrAppendEnv(envs, "PATH", newPath)

	if err := os.Setenv("PATH", newPath); err != nil {
		return []string{}, err
	}

	return envs, nil
}

func (l *Localfilecoin) signalAndWait(p *os.Process, waitch <-chan struct{}, signal os.Signal, t time.Duration) error {
	err := p.Signal(signal)
	if err != nil {
		return fmt.Errorf("error killing daemon %s: %s", l.dir, err)
	}

	select {
	case <-waitch:
		return nil
	case <-time.After(t):
		return errTimeout
	}
}

func (l *Localfilecoin) readerFor(file string) (io.ReadCloser, error) {
	return os.OpenFile(filepath.Join(l.dir, file), os.O_RDONLY, 0)
}

// GetPeerID returns the nodes peerID by running its `id` command.
// TODO this a temp fix, should read the nodes keystore instead
func (l *Localfilecoin) GetPeerID() (cid.Cid, error) {
	// run the id command
	out, err := l.RunCmd(context.TODO(), nil, l.binPath, "id", "--format=<id>")
	if err != nil {
		return cid.Undef, err
	}

	stdout, stderr, err := readAllOutput(context.Background(), out)
	if err != nil {
		return cid.Undef, errors.Wrapf(err, string(stderr))
	}

	rawcid := string(stdout)
	cidStr := strings.TrimSpace(rawcid)

	// decode the parsed string to a cid...maybe
	return cid.Decode(cidStr)
}
