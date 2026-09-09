// Copyright 2026 InsightOS
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package robotruntime

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Catalog 按 Robot 型号、后端和后端配置精确选择运行 Bundle。
// 同一型号可以同时安装 Fake、MuJoCo 和真机 Bundle，但不能靠注册顺序决定。
type Catalog struct {
	mu      sync.RWMutex
	bundles []Bundle
}

type bundleManifest struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"metadata"`
	Spec struct {
		Robot struct {
			Model           string `yaml:"model"`
			BackendProfiles []struct {
				Backend string `yaml:"backend"`
				Profile string `yaml:"profile"`
			} `yaml:"backendProfiles"`
		} `yaml:"robot"`
	} `yaml:"spec"`
}

// LoadCatalog 从只读 Bundle Store 加载类型包。Framework 只读取包身份和
// model/backend/profile 匹配条件；进程路径、Wheel 和 Ability 清单仍由
// semantic-robot-instance 解释，避免 Server 再复制一套部署格式。
func LoadCatalog(root string) (*Catalog, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("Robot Runtime Bundle Store 路径必填")
	}
	var bundles []Bundle
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || entry.Name() != "bundle.yaml" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var manifest bundleManifest
		if err := yaml.Unmarshal(content, &manifest); err != nil {
			return fmt.Errorf("解析 %s: %w", path, err)
		}
		if manifest.Kind != "RobotRuntimeBundle" || strings.TrimSpace(manifest.Metadata.Name) == "" ||
			strings.TrimSpace(manifest.Metadata.Version) == "" || strings.TrimSpace(manifest.Spec.Robot.Model) == "" {
			return fmt.Errorf("%s 不是有效 RobotRuntimeBundle", path)
		}
		for _, profile := range manifest.Spec.Robot.BackendProfiles {
			bundles = append(bundles, Bundle{Name: manifest.Metadata.Name, Version: manifest.Metadata.Version,
				Path: filepath.Dir(path), Match: MatchKey{RobotModel: manifest.Spec.Robot.Model,
					Backend: profile.Backend, BackendProfile: profile.Profile}})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("加载 Robot Runtime Bundle Store: %w", err)
	}
	if len(bundles) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrBundleNotFound, root)
	}
	return NewCatalog(bundles...)
}

func NewCatalog(bundles ...Bundle) (*Catalog, error) {
	result := &Catalog{}
	for _, bundle := range bundles {
		if err := result.Register(bundle); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (c *Catalog) Register(bundle Bundle) error {
	if strings.TrimSpace(bundle.Name) == "" || strings.TrimSpace(bundle.Version) == "" {
		return fmt.Errorf("Runtime Bundle name 和 version 必填")
	}
	if err := validateMatch(bundle.Match); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, existing := range c.bundles {
		if existing.Name == bundle.Name && existing.Version == bundle.Version &&
			existing.Match == bundle.Match {
			return fmt.Errorf("Runtime Bundle %s@%s 已注册", bundle.Name, bundle.Version)
		}
	}
	c.bundles = append(c.bundles, cloneBundle(bundle))
	return nil
}

func (c *Catalog) Resolve(match MatchKey) (Bundle, error) {
	if err := validateMatch(match); err != nil {
		return Bundle{}, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result *Bundle
	for _, bundle := range c.bundles {
		if bundle.Match != match {
			continue
		}
		if result != nil {
			return Bundle{}, fmt.Errorf("%w: %+v", ErrBundleAmbiguous, match)
		}
		item := cloneBundle(bundle)
		result = &item
	}
	if result == nil {
		return Bundle{}, fmt.Errorf("%w: %+v", ErrBundleNotFound, match)
	}
	return *result, nil
}

// ResolveDescriptor 是 Server Orchestrator 的正式入口，必须完整匹配
// Model、Backend 和 BackendProfile。
func (c *Catalog) ResolveDescriptor(descriptor VirtualRobotDescriptor) (Bundle, error) {
	return c.Resolve(descriptor.MatchKey())
}
func validateMatch(match MatchKey) error {
	if strings.TrimSpace(match.RobotModel) == "" || strings.TrimSpace(match.Backend) == "" ||
		strings.TrimSpace(match.BackendProfile) == "" {
		return fmt.Errorf("Runtime Bundle 匹配需要 robot_model、backend 和 backend_profile")
	}
	return nil
}

func cloneBundle(source Bundle) Bundle {
	return source
}
