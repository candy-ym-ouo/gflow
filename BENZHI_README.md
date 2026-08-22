# GFlow Workflow Engine

GFlow 是一个配置驱动的 Go 工作流引擎，提供工作流定义校验、实例推进、审批任务、动作执行、失败恢复、事件查询和简单 Web 界面。

## 本地运行

```bash
go test ./...
go vet ./...
go build ./...
go run ./cmd/server
```

启动后访问：

- `http://localhost:8080/ui/`
- `http://localhost:8080/api/v1/healthz`

## Docker 验证

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh gflow linux/amd64
./build_benzhi_docker.sh gflow linux/arm64
docker run -it gflow:latest
```

容器内可继续执行 `go test ./...`、`go vet ./...` 和 `go build ./...`。项目使用内存存储，服务重启后运行数据不会保留。
