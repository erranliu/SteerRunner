# SteerRunner

SteerRunner 是从固定 Spec / Plan 执行到可核验 PR 的独立工具。计划以一个 Windows EXE 提供常驻运行服务与 CLI 客户端；CLI 通过本机 IPC 调用服务，执行、观察和恢复由服务负责。

本仓库当前处于首版契约设计阶段，尚无可运行程序。文档中的命令是待实现的公共契约，不能视为已有能力。

- [首版产品与 CLI 契约](docs/spec/steerrunner.spec.md)
- [跨仓拆分实施计划与最小测试安排](docs/spec/steerrunner.plan.md)
- [专属验收要求与结果登记](docs/acceptance/steerrunner.md)

首版衔接 SteerLine 的执行三层重构，提取 Workflow、ReliableExecution、ExecutionAdapter 及独立宿主。SteerLine 保留任务规划、DAG、候选关系和最终 TaskDelivery，通过 CLI 消费 SteerRunner 的运行事实。

独立项目默认资源内嵌。任务材料、仓库、认证及具体执行后端需要的 Git / 代理工具是执行输入与环境依赖。OpenHands、SWE-ReX 或持久化 workflow 引擎的集成可以后续改变内部实现，不改变本版对外职责。
