# BattleMind Analyzer

一个基于 Go 实现的最小战斗分析服务：接收规范化战斗输入，调用 LLM，返回结构化分析结果。

## 项目目标

- 打通最小闭环：`battle report -> AnalyzeRequest -> /analyze -> 结构化结果`
- 提供一个本地可运行、可调试、可重复验证的分析服务
- 为后续规则校验、前端展示、诊断排序提供稳定输入输出协议

## 当前已实现能力

- `GET /health`：最小健康检查
- `POST /analyze`：分析接口
- `POST /tools/convert/analyze-request`：battle report 转 AnalyzeRequest
- `cmd/convertbatch`：批量转换 battle report
- 配置文件加载：`config.json`
- 最小 LLM client：`net/http` + timeout + 错误处理
- `/analyze` 固定 JSON 输出：`summary` / `issues` / `suggestions`
- 兼容旧格式 `problems -> issues`
- 模型返回非法 JSON 时，支持一次 repair 重试
- `/analyze` 请求级日志：`request_id` / `duration_ms` / `model_name` / `error_reason`
- 日志同时输出到控制台和文件
- 一键样例验证脚本：`scripts/test_analyze.sh`

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
- `cmd/convertbatch`：批量转换工具
- `internal/config`：配置读取与默认值处理
- `internal/handler`：HTTP 层，包含 `health` / `analyze` / `convert`
- `internal/service`：分析、转换、结果解析流程
- `internal/llm`：模型调用封装
- `internal/logging`：标准库日志输出到控制台和文件
- `internal/model`：请求、响应、battle report 结构定义
- `scripts`：本地脚本入口
- `testdata`：请求样例和 battle report 样例

## 配置说明

服务默认读取项目根目录的 `config.json`。仓库提供了 [config.json.example](d:/github.com/battlemind/config.json.example) 作为模板。

最小配置示例：

```json
{
  "server": {
    "port": 8080
  },
  "logging": {
    "file_path": "logs/server.log"
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

- `model.api_key` / `model.base_url` / `model.model` 为必填
- `model.timeout_seconds <= 0` 时默认使用 `30`
- `logging.file_path` 可选，默认是 `logs/server.log`
- 日志会同时写到控制台和日志文件
- 不要提交真实 `config.json` 和真实密钥

## 快速启动

1. 准备配置文件

```bash
cp config.json.example config.json
```

Windows PowerShell：

```powershell
Copy-Item config.json.example config.json
```

2. 启动服务

```bash
go run ./cmd/server
```

或：

```bash
make server
```

3. 检查健康状态

```bash
curl http://localhost:8080/health
```

预期返回：

```json
{
  "ok": true
}
```

## API 示例

### `POST /analyze`

当前 `/analyze` 支持两类输入：

- 旧方式：包含 `log_text`
- 当前主方式：传入结构化 `AnalyzeRequest`

结构化请求示例：

```bash
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  --data '{
    "metadata": {
      "battle_type": "baseline",
      "build_tags": ["dot", "single"],
      "notes": "phase2 dps drop"
    },
    "summary": {
      "win": true,
      "duration": 78,
      "likely_reason": "rotation efficiency is low"
    },
    "metrics": {
      "damage_by_source": {
        "dot": 120.5,
        "direct": 80,
        "basic_attack": 12
      },
      "skill_usage": {
        "contagion_wave": 9,
        "rupture_bloom": 5
      }
    },
    "diagnosis": [
      {
        "code": "LOW_SURVIVAL",
        "severity": "warn",
        "message": "hp is too low"
      }
    ]
  }'
```

返回示例：

```json
{
  "ok": true,
  "data": {
    "summary": "战斗获胜，但循环效率不足，DOT 占比偏低。",
    "issues": [
      {
        "title": "DOT 占比偏低",
        "description": "当前构筑预期依赖 DOT 输出，但实际表现不足。",
        "severity": "medium",
        "evidence": [
          "DOT 伤害占比偏低",
          "普攻占比偏高"
        ]
      }
    ],
    "suggestions": [
      "优化 DOT 技能覆盖",
      "减少普攻填充"
    ],
    "raw_text": "{...model raw output...}"
  }
}
```

错误返回示例：

```json
{
  "error": {
    "code": "EMPTY_LOG_TEXT",
    "message": "log_text or structured analyze input is required"
  }
}
```

当前常见错误码：

- `INVALID_JSON`
- `EMPTY_LOG_TEXT`
- `LOG_TOO_LONG`
- `INVALID_BATTLE_TYPE`
- `INVALID_BUILD_TAGS`
- `NOTES_TOO_LONG`
- `INVALID_MODEL_JSON`
- `ANALYZE_FAILED`

### `POST /tools/convert/analyze-request`

把 battle report JSON 转成 `/analyze` 可直接消费的 AnalyzeRequest：

```bash
curl -X POST http://localhost:8080/tools/convert/analyze-request \
  -H "Content-Type: application/json" \
  --data @testdata/battle-report/battle-report.json
```

下载模式：

```bash
curl -X POST "http://localhost:8080/tools/convert/analyze-request?download=1" \
  -H "Content-Type: application/json" \
  --data @testdata/battle-report/battle-report.json \
  -o battle-report.analyze_request.json
```

## 请求日志

每次 `/analyze` 请求结束时，都会输出一条统一格式的请求日志，并回传响应头 `X-Request-ID`。

日志至少包含：

- `request_id`
- `duration_ms`
- `model_name`
- `error_reason`

当前日志文件默认写入：

```text
logs/server.log
```

日志示例：

```text
component=analyze_request_log event=request_completed request_id=analyze-1775551201000000000 duration_ms=842 model_name="deepseek-chat" error_reason="NONE" success=true status_code=200 method=POST path=/analyze battle_type="baseline" log_text_length=0
```

## 样例测试脚本

项目提供了一个最小脚本，用来一键验证 `/analyze` 的关键样例。

脚本入口：

```bash
bash scripts/test_analyze.sh
```

Makefile 入口：

```bash
make test-sample
```

默认覆盖 5 组样例：

- `normal`
- `short-log`
- `long-log`
- `invalid-input`
- `timeout-sim`

脚本行为：

- 先检查 `/health`
- 逐个调用 `/analyze`
- 检查状态码和关键字
- 输出每组 `PASS` / `FAIL`
- 最后输出汇总
- 有失败时返回非 0

超时样例通过请求头模拟：

```text
X-Debug-Simulate-Timeout: 1
```

说明：

- 脚本依赖 `bash`、`curl`、`grep`、`sed`
- 在 Windows 上建议使用 Git Bash 或 WSL
- 如果要故意验证失败出口，可执行：

```bash
FORCE_FAIL=1 bash scripts/test_analyze.sh
```

## 运行与验证命令

```bash
# 启动服务
go run ./cmd/server

# 健康检查
curl http://localhost:8080/health

# 分析调用
curl -X POST http://localhost:8080/analyze \
  -H "Content-Type: application/json" \
  --data @testdata/analyze_request/battle-report.analyze_request.json

# 批量转换
go run ./cmd/convertbatch -input testdata/battle-report -output-dir testdata/analyze_request

# 运行 Go 测试
go test ./...

# 运行样例测试脚本
bash scripts/test_analyze.sh
```

使用 Makefile：

```bash
make server
make test
make test-sample
make convert INPUT=path/to/battle-report OUTPUT_DIR=path/to/out
```

## 已知限制

- 当前分析质量仍依赖模型输出稳定性
- 只做了最小 JSON 容错和一次 repair 重试
- 结果字段仍然偏轻量，后续还可以继续扩展
- 样例测试脚本是最小验证入口，不是完整集成测试平台
- Windows PowerShell 默认的 `curl` 是 `Invoke-WebRequest` 别名，文件上传建议使用 `curl.exe` 或 Git Bash
