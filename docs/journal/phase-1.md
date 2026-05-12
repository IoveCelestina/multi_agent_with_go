# 阶段 1

## 最难的决定

最难的是 provider streaming 的错误边界。`Stream` 启动前失败直接返回 `error`，流已经开始后的协议错误走 `EventError`，而调用方主动 `ctx` 取消时允许静默关闭 channel。这个选择让 CLI 的 `Ctrl+C` 行为更自然，也避免把用户主动取消误报成运行失败。

另一个关键点是 tool call delta 的归一化。DeepSeek/OpenAI-compatible API 会把 function name 和 arguments 分块吐出，AgentRunner 不应该看见这些半成品。因此 provider 内部按 index assemble，只有 finish 或 `[DONE]` 时才发完整 `ToolCall`。

## 和预期不同的地方

一开始为了保持零依赖手写了一个 YAML 子集解析器，实际很快变成噪声。配置解析不是 agent 工程核心，继续维护会拖累后面的 agents/tools/workflows。阶段 1 后决定改用 `gopkg.in/yaml.v3`，把注意力留给 provider、tool loop、event log 和状态机。

流式 HTTP 也比预期更容易踩坑。`http.Client.Timeout` 会限制整个请求生命周期，包括读 body；这对长流式响应不合适。更合理的是不设置整体 timeout，只用 `ResponseHeaderTimeout` 限制建立连接和等待响应头，真正的取消交给 context。

## 对照其他项目

OpenAI Swarm 的价值在于小：agent handoff 和 tool calling 是清楚的普通控制流，而不是一开始就抽象成复杂框架。这个项目也应该保持类似方向：Provider、Tool、AgentRunner 的边界要朴素、可测试。

LangGraph 更适合在 Coordinator 和 resume 阶段对照。它的核心价值是状态图和可恢复执行，不是 provider streaming。当前阶段先不急着引入图抽象，等阶段 4/7 再对照它的状态模型会更有收益。

Claude Code SDK 一类项目更值得对照 streaming、tool result 和错误处理风格。阶段 1 暴露出来的坑，比如 `[DONE]` 兜底、tool call index 排序、取消语义，都是后续 AgentRunner 稳定性的地基。
