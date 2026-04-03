# BattleMind Analyzer

一个基于 Go 的最小战斗分析服务：输入规范化战斗数据，调用 LLM，输出结构化分析结果。

## 项目目标
- 在第一周打通最小闭环：`规范化输入 -> 模型分析 -> 固定 JSON 输出`
- 提供可本地运行、可通过 HTTP 调用的后端服务
- 为后续规则校验、前端展示和诊断排序提供稳定数据结构

## 当前已实现能力
- `GET /health`：健康检查
- `POST /analyze`：分析接口（触发真实模型调用）
- `POST /tools/convert/analyze-request`：battle report 转 AnalyzeRequest（独立工具接口）
- 配置文件加载（`config.json`）
- 最小 LLM client（`net/http` + timeout + 错误处理）
- `/analyze` 结构化输出：`summary` / `issues` / `suggestions`（含调试字段 `raw_text`）
- 基础容错：支持解析模型输出中的 ```json 代码块包裹

## 当前暂不做什么
- 不做前端页面
- 不做数据库持久化
- 不做登录鉴权
- 不做复杂规则引擎
- 不做多模型调度框架
- 不做 RAG / function calling / agent 自修复
- 不做生产级部署与监控体系

## 仓库结构
- `cmd/server`：HTTP 服务启动入口
- `cmd/convertbatch`：本地批量转换工具（battle report -> analyze_request）
- `internal/config`：配置读取与校验
- `internal/handler`：HTTP 层（health / analyze / convert）
- `internal/service`：业务流程（prompt 构造、结果解析、转换逻辑）
- `internal/llm`：模型调用封装
- `internal/model`：请求/响应结构定义
- `testdata`：测试数据目录（默认空骨架）

## 配置说明
服务默认读取项目根目录 `config.json`。仓库提交了 [config.json.example](./config.json.example) 作为模板。

最小配置如下：

```json
{
  "server": {
    "port": 8080
  },
  "model": {
    "api_key": "your-api-key",
    "base_url": "https://api.openai.com/v1",
    "model": "gpt-4.1-mini",
    "timeout_seconds": 30
  }
}
```

说明：
- `api_key` / `base_url` / `model` 为必填
- `timeout_seconds <= 0` 时默认 `30`
- 请勿提交真实 `config.json` 与真实密钥

## 快速启动
1. 准备配置文件

```bash
cp config.json.example config.json
```

Windows PowerShell 可用：

```powershell
Copy-Item config.json.example config.json
```

2. 启动服务

```bash
go run ./cmd/server
```

如果本机有 `make`，也可使用：

```bash
make server
```

3. 检查健康状态

```bash
curl http://localhost:8080/health
```

## API 示例

### 1) `GET /health`

请求：

```bash
curl http://localhost:8080/health
```

响应：

```json
{
  "ok": true
}
```

### 2) `POST /analyze`

请求（最小示例）：
`schema_version` 省略时默认按 `v1` 处理。

```bash
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  --data '{
    "schema_version": "v1",
    "metadata": {
      "battle_type": "baseline",
      "build_tags": ["dot", "single"],
      "floor_id": "floor-883",
      "notable_rules": ["no_heal"],
      "floor_modifiers": ["low_mana"],
      "notes": "custom note"
    },
    "summary": {
      "win": true,
      "duration": 78,
      "likely_reason": "rotation breaks in late phase"
    },
    "metrics": {
      "damage_by_source": {
        "dot": 120.5,
        "direct": 80.0,
        "basic_attack": 20.0
      },
      "skill_usage": {
        "contagion_wave": 9,
        "toxic_lance": 17
      }
    },
    "diagnosis": [
      {
        "code": "LOW_DOT_RATIO",
        "severity": "warn",
        "message": "dot ratio is low",
        "details": {"dotRatio": 0.41}
      }
    ]
  }'
```

响应（当前真实外层包装）：

```json
{
  "ok": true,
  "data": {
    "summary": "后半段输出下降，循环稳定性不足。",
    "issues": [
      {
        "title": "DOT 覆盖率偏低",
        "description": "DOT 输出占比偏低，核心输出机制未充分发挥。",
        "severity": "medium",
        "evidence": ["DOT 覆盖率偏低", "普攻占比偏高"]
      }
    ],
    "suggestions": [
      "优先保证 DOT 技能覆盖",
      "减少普攻填充，优化资源循环"
    ],
    "raw_text": "{...模型原始文本...}"
  }
}
```

### 3) `POST /tools/convert/analyze-request`（可选）

把 battle report JSON 转为 `/analyze` 入参：

```bash
curl -X POST http://localhost:8080/tools/convert/analyze-request \
  -H "Content-Type: application/json" \
  --data @your-battle-report.json
```

## 输入输出示例（最小闭环）
- 输入：`AnalyzeRequest`（规范化战斗输入）
- 输出：`AnalyzeResult`（结构化分析结果）
  - `summary`: 一句话结论
  - `issues`: 结构化问题列表（title / description / severity / evidence）
  - `suggestions`: 建议列表
  - `raw_text`: 调试阶段保留的模型原始文本（可选）

## 运行与验证命令

```bash
# 启动服务
go run ./cmd/server

# 健康检查
curl http://localhost:8080/health

# 分析调用（文件输入）
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  --data @your-analyze-request.json
```

如果你使用 `make`：

```bash
make server
make test
make convert INPUT=path/to/battle-report OUTPUT_DIR=path/to/out
```

## 已知限制
- 当前结果质量依赖模型输出稳定性
- 仅做了最小 JSON 容错（空白、代码块、首个 JSON 对象提取）
- 未做复杂重试策略和 provider 抽象
- 未做长日志分片与成本优化
- 结构化字段仍较少，后续会继续扩展
