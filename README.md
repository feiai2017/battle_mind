# BattleMind Analyzer

## v0.1 范围说明

### 项目当前目标
在第一周完成最小闭环：`日志文本 -> 调模型 -> 返回固定 JSON`。

### v0.1 做什么
- 提供一个最小后端服务
- 接收战斗日志文本
- 调用一次大模型完成分析
- 返回固定 JSON 结构

### v0.1 不做什么
- 不做前端页面
- 不做数据库持久化
- 不做登录鉴权/用户系统
- 不做部署与监控体系
- 不做复杂规则引擎
- 不做多模型切换或抽象层
- 不做历史记录持久化

### 最小输入
一段战斗日志文本（纯文本）。

### 最小输出
固定 JSON，至少包含以下字段：
- `summary`
- `issues`
- `suggestions`
- `confidence`

### 本阶段完成标准
- 服务可以启动
- 提供健康检查接口
- 分析接口可以接收样例日志
- 接口返回合法 JSON（包含约定字段）

## 项目结构（骨架）
- `cmd/server`：程序入口
- `internal/config`：配置读取与管理
- `internal/handler`：HTTP 接口层
- `internal/service`：业务编排层
- `internal/llm`：模型调用封装
- `internal/model`：请求响应结构
- `testdata`：样例日志与本地测试数据

## 配置说明
- 启动默认读取项目根目录 `config.json`。
- 仓库只提交 `config.json.example`，不提交真实 `config.json`。
- 本地先复制 `config.json.example` 为 `config.json`，再填写真实模型配置。
- `model` 配置至少包含：`api_key`、`base_url`、`model`、`timeout_seconds`。
