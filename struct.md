```
ainex-backend/
├── go.mod                     # 全局依赖管理
├── Makefile                   # 包含编译、Wire 生成、Docker 镜像构建指令
├── cmd/
│   └── ainex-app/             # 【程序入口模块】
│       ├── main.go            # 只有极简的启动逻辑：加载配置 -> 调用 InitApp -> 运行
│       ├── wire.go            # 依赖注入声明（指挥官）
│       └── wire_gen.go        # Wire 自动生成的组装代码
│
├── configs/                   # 【配置模块】
│   ├── config.yaml            # 生产环境配置
│   └── config.dev.yaml        # 开发环境配置
│
├── internal/                  # 【私有业务核心】
│   ├── bootstrap/             # 【点火模块】负责系统初始化流程（路由注册、优雅停机）
│   │   └── bootstrap.go
│   │
│   ├── common/                # 【项目公共规范】
│   │   ├── xerr/              # 统一错误码
│   │   ├── response/          # 统一 JSON 返回格式
│   │   └── middleware/        # JWT、日志、跨域中间件
│   │
│   ├── infrastructure/        # 【基础设施层】技术实现（对接各种中间件）
│   │   ├── persistence/       # 持久化：mysql.go, es.go, mongo.go
│   │   ├── cache/             # 缓存：redis.go
│   │   ├── mq/                # 消息队列：kafka.go, rabbitmq.go
│   │   └── ai/                # 大模型：client.go, openai.go, deepseek.go
│   │
│   # --- 业务领域模块 (按功能拆分，物理隔离) ---
│   ├── user/                  # 【用户模块】
│   │   ├── handler/           # 控制层：处理 HTTP 请求 (原 api 目录)
│   │   ├── service/           # 业务层：逻辑编排
│   │   ├── domain/            # 领域层：实体定义与接口契约
│   │   ├── repository/        # 持久层：数据库具体操作
│   │   └── dto/               # 传输对象：请求/响应结构体 (避开 domain 污染)
│   │
│   ├── record/                # 【打卡模块】
│   │   └── ... (内部结构同 user)
│   │
│   └── note/                  # 【笔记模块】
│       └── ... (内部结构同 user)
│
├── pkg/                       # 【外部公共库】不含业务逻辑的工具包（可被其他项目引用）
│   ├── xlog/                  # 日志工具封装
│   └── util/                  # 纯工具函数（字符串、时间等）
│
├── scripts/                   # 【脚本】SQL 迁移、部署脚本、初始化数据
├── api/                       # 【协议定义】Swagger UI, Protobuf 文件
└── test/                      # 【集成测试】跨模块的测试用例
```