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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"insightos.cn/semantic-framework/internal/simulation"
)

func TestActivateRuntimeInstallationCopiesSceneResources(t *testing.T) {
	root := t.TempDir()
	paths := runtimePaths{
		runtimes: filepath.Join(root, "configs", "runtimes.d"),
		scenes:   filepath.Join(root, "configs", "scenes.d"),
	}
	for _, dir := range []string{paths.runtimes, paths.scenes} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	pack := filepath.Join(root, "runtime-packs", "native-mujoco", "0.4.0")
	catalog := `schema_version: 1
catalog_version: 0.4.0
entries:
  - scene_id: demo
    name: Demo
    engine: mujoco
    loader: native
    compatible_runtime_profile: native-mujoco
    versions:
      - version: 1.0.0
        runtime_scene_key: demo
        published: true
        robot_models: [r1]
        variants: [{variant_id: layout001, name: Layout 001, kind: layout}]
        authoring: {mode: none}
`
	for path, data := range map[string]string{
		"catalog/catalog.yaml": catalog, "catalog/previews/demo.svg": "<svg/>",
	} {
		absolute := filepath.Join(pack, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := simulation.RuntimePackManifest{
		SceneCatalog:   simulation.RuntimePackFile{Path: "catalog/catalog.yaml"},
		SceneResources: []simulation.RuntimePackFile{{Path: "catalog/previews/demo.svg"}},
	}
	scenePath := filepath.Join(paths.scenes, "local-native", "catalog.yaml")
	installation := simulation.RuntimeInstallation{
		SchemaVersion: 2, InstallationID: "local-native",
		Profile: simulation.RuntimeProfile{RuntimeProfileID: "native-mujoco", Name: "Native",
			Engine: "mujoco", Loader: "native", APIVersion: "v1"},
		PackID: "native-mujoco", PackVersion: "0.4.0", Runner: "native-mujoco",
		LaunchMode: "process", EnvironmentPath: filepath.Join(root, "runtime-envs", "local-native", "0.4.0"),
		PackPath: pack, SceneCatalogPath: scenePath, Endpoint: "http://127.0.0.1:18090",
		Enabled: true, InstalledVersion: "0.4.0",
	}
	if err := activateRuntimeInstallation(paths, installation, manifest, pack, false); err != nil {
		t.Fatalf("激活 installation 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(scenePath), "previews", "demo.svg")); err != nil {
		t.Fatalf("场景资源未安装: %v", err)
	}
	loaded, err := simulation.LoadSceneCatalog(paths.scenes)
	if err != nil {
		t.Fatal(err)
	}
	items := loaded.List("native-mujoco")
	if len(items) != 1 || items[0].SceneID != "demo" {
		t.Fatalf("catalog=%+v", items)
	}
	if _, err := simulation.LoadRuntimeInstallationFile(filepath.Join(paths.runtimes, "local-native.yaml")); err != nil {
		t.Fatalf("生成的 installation 无法重新加载: %v", err)
	}
}

func TestInstallationIDAndPathContainment(t *testing.T) {
	if installationIDValid("../escape") || installationIDValid("Upper") || !installationIDValid("local-native.1") {
		t.Fatal("installation id 校验错误")
	}
	root := t.TempDir()
	if !pathInside(root, filepath.Join(root, "child")) || pathInside(root, root) || pathInside(root, filepath.Dir(root)) {
		t.Fatal("卸载路径边界校验错误")
	}
}

func TestRuntimePackArchiveEntryAndExtractedTreeSafety(t *testing.T) {
	for _, value := range []string{"catalog/", "catalog/catalog.yaml", "wheels/runtime.whl"} {
		if err := validateRuntimeArchiveEntry(value); err != nil {
			t.Fatalf("正常 Pack 路径 %q 被拒绝: %v", value, err)
		}
	}
	for _, value := range []string{"../escape", "/absolute", "catalog//item", "C:/pack"} {
		if err := validateRuntimeArchiveEntry(value); err == nil {
			t.Fatalf("不安全 Pack 路径 %q 未被拒绝", value)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "regular"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateExtractedRuntimePackTree(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "regular"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := validateExtractedRuntimePackTree(root); err == nil {
		t.Fatal("Runtime Pack 中的符号链接必须被拒绝")
	}
}

func TestReleaseDoctorRejectsDevelopmentPackBeforeRuntimeProbe(t *testing.T) {
	item := simulation.RuntimeInstallation{
		PackVersion: "0.4.0-dev.0", Development: false, Enabled: true,
	}
	err := diagnoseRuntimeInstallation(item, false, true)
	if err == nil || err.Error() != "开发版本 Runtime Pack 不得用于 RC 或正式制品验收" {
		t.Fatalf("err=%v", err)
	}
}
