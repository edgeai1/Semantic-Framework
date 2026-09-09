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

package simulation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// 原生 MuJoCo 首次解析多箱场景和编译渲染资源时可能超过一分钟。
	// HTTP 启动请求已经返回 starting，真正负责后续 Map 首次同步和受管 Robot
	// 拉起的是这个观察协程；观察窗口短于 Robot 启动窗口会造成“场景稍后可用，
	// 但 Robot 永远不再启动”的断链。这里与受管实例外层启动窗口统一为三分钟，
	// 只延长对同一 instance/generation 的观察，不重试或重放任何物理命令。
	catalogStartObserveTimeout = 3 * time.Minute
	catalogStartPollInterval   = 100 * time.Millisecond
)

// CatalogSceneStartRequest 只包含 Project 场景详情页允许选择的启动参数。
// Runtime Profile、scene_key 和评测能力均由服务端目录解析，浏览器不能覆盖。
type CatalogSceneStartRequest struct {
	RequestID             string `json:"request_id"`
	SceneVersion          string `json:"scene_version"`
	VariantID             string `json:"variant_id"`
	RuntimeInstallationID string `json:"runtime_installation_id,omitempty"`
	Seed                  int64  `json:"seed"`
	Headless              bool   `json:"headless"`
	RenderBackend         string `json:"render_backend"`
}

func (s *Service) ValidateRuntimeInstallation(installationID string) error {
	installation, err := s.RuntimeInstallation(strings.TrimSpace(installationID))
	if err != nil {
		return err
	}
	return validateInstallationAvailable(installation)
}

func (s *Service) SceneCatalogVersion(
	sceneID, versionID string,
) (SceneCatalogEntry, SceneCatalogVersion, error) {
	if s.sceneCatalog == nil {
		return SceneCatalogEntry{}, SceneCatalogVersion{},
			errors.New("SceneCatalog 尚未配置")
	}
	return s.sceneCatalog.Version(sceneID, versionID)
}

// CreateProjectLayoutDraft 从 Project 已引用的公共场景创建 Layout 草稿。
// 目录源文件在 Server 启动时已经严格解析；浏览器只选择初始化方式和名称，
// 不能上传任意 SceneDocument 冒充公共模板。
func (s *Service) CreateProjectLayoutDraft(
	authoring *SceneAuthoringService,
	projectID, projectSceneID, sceneID, versionID, sourceVariantID,
	initialization, name string,
) (SceneDocument, error) {
	if authoring == nil || s.sceneCatalog == nil {
		return SceneDocument{}, errors.New("场景编辑服务尚未配置")
	}
	entry, version, variant, err := s.ValidateProjectCatalogScene(
		projectID, sceneID, versionID, sourceVariantID,
	)
	if err != nil {
		return SceneDocument{}, err
	}
	variantID := variant.VariantID
	lookupVariantID := variantID
	if initialization == "empty_layout" {
		lookupVariantID = ""
	}
	source, err := s.sceneCatalog.AuthoringDocument(sceneID, versionID, lookupVariantID)
	if err != nil {
		return SceneDocument{}, err
	}
	return authoring.CreateProjectLayout(
		projectID, projectSceneID, name, initialization, variantID,
		entry, version, source,
	)
}

// ValidateProjectCatalogScene 是“添加到 Project”和“启动”的共同检查。
// 返回值中的 variant 已按目录验证；只读 benchmark 场景不会因此变成可编辑草稿。
func (s *Service) ValidateProjectCatalogScene(
	projectID, sceneID, versionID, variantID string,
) (SceneCatalogEntry, SceneCatalogVersion, SceneCatalogVariant, error) {
	entry, version, err := s.SceneCatalogVersion(sceneID, versionID)
	if err != nil {
		return SceneCatalogEntry{}, SceneCatalogVersion{}, SceneCatalogVariant{}, err
	}
	profileID, _, err := s.projectRuntimePreference(projectID)
	if err != nil {
		return SceneCatalogEntry{}, SceneCatalogVersion{}, SceneCatalogVariant{}, err
	}
	if profileID != "" && profileID != entry.CompatibleRuntimeProfile {
		return SceneCatalogEntry{}, SceneCatalogVersion{}, SceneCatalogVariant{},
			fmt.Errorf("%w: 场景 %s 需要 %s，Project 默认 Profile 是 %s", ErrConflict,
				entry.SceneID, entry.CompatibleRuntimeProfile,
				profileID)
	}
	variant, err := selectCatalogVariant(version, variantID)
	if err != nil {
		return SceneCatalogEntry{}, SceneCatalogVersion{}, SceneCatalogVariant{}, err
	}
	profile, profileErr := s.runtimeProfile(entry.CompatibleRuntimeProfile)
	if profileErr != nil {
		return SceneCatalogEntry{}, SceneCatalogVersion{}, SceneCatalogVariant{}, profileErr
	}
	if version.Evaluation != nil && !profile.Capabilities.NativeEvaluator {
		return SceneCatalogEntry{}, SceneCatalogVersion{}, SceneCatalogVariant{},
			fmt.Errorf("%w: 场景声明了评测，但绑定 Runtime 不提供 evaluator", ErrConflict)
	}
	return entry, version, variant, nil
}

func selectCatalogVariant(
	version SceneCatalogVersion, variantID string,
) (SceneCatalogVariant, error) {
	if variantID == "" && len(version.Variants) > 0 {
		variantID = version.Variants[0].VariantID
	}
	for _, variant := range version.Variants {
		if variant.VariantID == variantID {
			return variant, nil
		}
	}
	return SceneCatalogVariant{}, fmt.Errorf("%w: 场景版本不存在 variant %s",
		ErrNotFound, variantID)
}

// StartCatalogScene 从只读目录解析 Runtime 输入。这样 Studio 不能把另一个引擎
// 的 scene_key、未发布版本或伪造的 evaluator 塞入 Project Runtime。
func (s *Service) StartCatalogScene(
	ctx context.Context, projectID, sceneID string, request CatalogSceneStartRequest,
) (SceneInstance, error) {
	entry, version, variant, err := s.ValidateProjectCatalogScene(
		projectID, sceneID, request.SceneVersion, request.VariantID)
	if err != nil {
		return SceneInstance{}, err
	}
	sceneKey := version.RuntimeSceneKey
	layout := variant.VariantID
	if variant.RuntimeSceneKey != "" {
		sceneKey = variant.RuntimeSceneKey
	}
	if variant.RuntimeLayout != "" {
		layout = variant.RuntimeLayout
	}
	instance, err := s.StartScene(ctx, projectID, sceneKey, SceneStartRequest{
		RequestID: request.RequestID, RuntimeProfileID: entry.CompatibleRuntimeProfile,
		RuntimeInstallationID: request.RuntimeInstallationID,
		RuntimeBundleID:       variant.RuntimeBundleID,
		Layout:                layout, Seed: request.Seed, Headless: request.Headless,
		RenderBackend: request.RenderBackend,
	})
	if err != nil {
		return SceneInstance{}, err
	}
	if s.projectBindings != nil {
		state, stateErr := s.state.LoadRuntimeState(projectID)
		if stateErr == nil && state.RuntimeInstallationID != "" {
			_ = s.projectBindings.RememberRuntimePreference(
				projectID, entry.CompatibleRuntimeProfile, state.RuntimeInstallationID,
			)
		}
	}
	s.stateMu.Lock()
	state, loadErr := s.state.LoadRuntimeState(projectID)
	if loadErr == nil {
		state.CatalogSceneID = entry.SceneID
		state.SceneVersion = version.Version
		state.VariantID = variant.VariantID
		state.Evaluation = version.Evaluation
		state.Revision++
		state.UpdatedAt = instance.UpdatedAt
		loadErr = s.state.SaveRuntimeState(state)
	}
	s.stateMu.Unlock()
	if loadErr != nil {
		_, _ = s.SceneOperation(ctx, projectID, instance.InstanceID, "stop", nil)
		return SceneInstance{}, loadErr
	}
	// Runtime 的模型加载是后台任务。starting 是有效的启动结果，不能在这里
	// 立即读取 Snapshot，否则真实 MuJoCo 会正确返回“实例尚不可操作”，并把
	// 已经创建的实例遗留在 starting。已经可读时同步首个地图检查点；仍在加载
	// 时由 Framework 自己观察状态，达到 running/paused 后再同步。
	if instance.State == "running" || instance.State == "paused" {
		if err := s.syncSceneCheckpoint(ctx, projectID, instance.InstanceID,
			"scene_start", true); err != nil {
			_, _ = s.SceneOperation(ctx, projectID, instance.InstanceID, "stop", nil)
			return SceneInstance{}, err
		}
		s.startSceneRobotsAsync(projectID, instance)
	} else if instance.State == "starting" {
		go s.observeCatalogSceneStart(projectID, instance)
	} else {
		return SceneInstance{}, fmt.Errorf("%w: Runtime 启动返回状态 %s",
			ErrConflict, instance.State)
	}
	return instance, nil
}

// observeCatalogSceneStart 只观察本次 instance/generation，不启动场景、不重放
// Robot 命令。请求 Context 在 HTTP 返回后会取消，因此这里使用有上限的独立
// Context；Project 退出、实例停止、generation 变化或 Runtime 失败都会终止观察。
func (s *Service) observeCatalogSceneStart(projectID string, started SceneInstance) {
	ctx, cancel := context.WithTimeout(context.Background(), catalogStartObserveTimeout)
	defer cancel()
	ticker := time.NewTicker(catalogStartPollInterval)
	defer ticker.Stop()

	for {
		instance, err := s.Scene(ctx, projectID, started.InstanceID)
		if err == nil {
			if instance.Generation != started.Generation {
				return
			}
			switch instance.State {
			case "running", "paused":
				if syncErr := s.syncSceneCheckpoint(
					ctx, projectID, started.InstanceID, "scene_start", true,
				); syncErr != nil {
					// 初始快照无法形成地图版本时，不允许保留一个看似可用的场景。
					// stop 仍会先让 Runtime 执行 hold，且不会重放任何历史命令。
					_, _ = s.SceneOperation(
						context.Background(), projectID, started.InstanceID, "stop", nil,
					)
				} else {
					s.startSceneRobotsAsync(projectID, instance)
				}
				return
			case "failed", "stopped", "stopping":
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Service) syncSceneCheckpoint(
	ctx context.Context, projectID, instanceID, reason string, newGeneration bool,
) error {
	snapshot, err := s.SceneSnapshot(ctx, projectID, instanceID)
	if err != nil {
		return err
	}
	return s.applySceneCheckpoint(ctx, projectID, snapshot, reason, newGeneration)
}

func (s *Service) applySceneCheckpoint(
	ctx context.Context,
	projectID string,
	snapshot SceneSnapshot,
	reason string,
	newGeneration bool,
) error {
	if s.mapSink == nil {
		return nil
	}
	if consumer, ok := s.mapSink.(SceneCheckpointConsumer); ok {
		return consumer.ApplySimulationCheckpoint(ctx, projectID, snapshot, reason, newGeneration)
	}
	return s.mapSink.ApplySimulationSnapshot(ctx, projectID, snapshot)
}

// SwitchCatalogVariant 只在同一已发布场景版本内切换 Layout/Init State。
// 旧实例先停止并失效，Runtime 进程保持；新实例和地图使用新的 generation。
func (s *Service) SwitchCatalogVariant(
	ctx context.Context, projectID, instanceID, variantID, requestID string, seed int64,
) (SceneInstance, error) {
	state, _, err := s.projectRuntime(projectID, instanceID)
	if err != nil {
		return SceneInstance{}, err
	}
	if state.CatalogSceneID == "" || state.SceneVersion == "" {
		return SceneInstance{}, fmt.Errorf("%w: 当前实例不是从 Project 场景引用启动", ErrConflict)
	}
	if _, _, _, err := s.ValidateProjectCatalogScene(projectID,
		state.CatalogSceneID, state.SceneVersion, variantID); err != nil {
		return SceneInstance{}, err
	}
	if err := s.syncSceneCheckpoint(ctx, projectID, instanceID,
		"layout_switch_checkpoint", false); err != nil {
		return SceneInstance{}, err
	}
	if _, err := s.SceneOperation(ctx, projectID, instanceID, "stop", nil); err != nil {
		return SceneInstance{}, err
	}
	if strings.TrimSpace(requestID) == "" {
		return SceneInstance{}, errors.New("request_id 不能为空")
	}
	return s.StartCatalogScene(ctx, projectID, state.CatalogSceneID,
		CatalogSceneStartRequest{
			RequestID: requestID, SceneVersion: state.SceneVersion,
			VariantID: variantID, Seed: seed, Headless: true,
			RenderBackend: "egl",
		})
}
