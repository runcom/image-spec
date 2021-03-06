// Copyright 2016 The Linux Foundation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package image

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/image-spec/schema"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

type cfg struct {
	User         string
	Memory       int64
	MemorySwap   int64
	CPUShares    int64 `json:"CpuShares"`
	ExposedPorts map[string]struct{}
	Env          []string
	Entrypoint   []string
	Cmd          []string
	Volumes      map[string]struct{}
	WorkingDir   string
}

type config struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Config       cfg    `json:"config"`
}

func findConfig(w walker, d *descriptor) (*config, error) {
	var c config
	cpath := filepath.Join("blobs", d.Digest)

	f := func(path string, info os.FileInfo, r io.Reader) error {
		if info.IsDir() {
			return nil
		}

		if filepath.Clean(path) != cpath {
			return nil
		}

		buf, err := ioutil.ReadAll(r)
		if err != nil {
			return errors.Wrapf(err, "%s: error reading config", path)
		}

		if err := schema.MediaTypeImageSerializationConfig.Validate(bytes.NewReader(buf)); err != nil {
			return errors.Wrapf(err, "%s: config validation failed", path)
		}

		if err := json.Unmarshal(buf, &c); err != nil {
			return err
		}

		return errEOW
	}

	switch err := w.walk(f); err {
	case nil:
		return nil, fmt.Errorf("%s: config not found", cpath)
	case errEOW:
		// found, continue below
	default:
		return nil, err
	}

	return &c, nil
}

func (c *config) runtimeSpec(rootfs string) (*specs.Spec, error) {
	if c.OS != "linux" {
		return nil, fmt.Errorf("%s: unsupported OS", c.OS)
	}

	var s specs.Spec
	s.Version = "0.5.0"
	s.Root.Path = rootfs
	s.Process.Cwd = c.Config.WorkingDir
	s.Process.Env = append([]string(nil), c.Config.Env...)
	s.Process.Args = append([]string(nil), c.Config.Entrypoint...)
	s.Process.Args = append(s.Process.Args, c.Config.Cmd...)

	if uid, err := strconv.Atoi(c.Config.User); err == nil {
		s.Process.User.UID = uint32(uid)
	} else if ug := strings.Split(c.Config.User, ":"); len(ug) == 2 {
		uid, err := strconv.Atoi(ug[0])
		if err != nil {
			return nil, errors.New("config.User: unsupported uid format")
		}

		gid, err := strconv.Atoi(ug[1])
		if err != nil {
			return nil, errors.New("config.User: unsupported gid format")
		}

		s.Process.User.UID = uint32(uid)
		s.Process.User.GID = uint32(gid)
	} else {
		return nil, errors.New("config.User: unsupported format")
	}

	s.Platform.OS = c.OS
	s.Platform.Arch = c.Architecture

	mem := uint64(c.Config.Memory)
	swap := uint64(c.Config.MemorySwap)
	shares := uint64(c.Config.CPUShares)

	s.Linux.Resources = &specs.Resources{
		CPU: &specs.CPU{
			Shares: &shares,
		},

		Memory: &specs.Memory{
			Limit:       &mem,
			Reservation: &mem,
			Swap:        &swap,
		},
	}

	for vol := range c.Config.Volumes {
		s.Mounts = append(
			s.Mounts,
			specs.Mount{
				Destination: vol,
				Type:        "bind",
				Options:     []string{"rbind"},
			},
		)
	}

	return &s, nil
}
