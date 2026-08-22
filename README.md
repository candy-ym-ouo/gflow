# GFlow

一个可运行的 Go 工作流引擎最小实现。项目按文档中的模块分层，提供工作流定义校验、内存持久化、异步执行、动作节点、审批节点、REST API 和静态 UI。

## 运行

```bash
go run ./cmd/server -addr :8080
```

打开 <http://localhost:8080/ui/>。服务默认使用内存存储，重启后数据会清空；这是开发模式，Store 接口可替换为 SQL 实现。

## API 示例

```bash
curl -X POST http://localhost:8080/api/v1/workflows \
  -H 'Content-Type: application/json' --data-binary @examples/simple.json
curl -X POST http://localhost:8080/api/v1/workflows/simple/instances \
  -H 'Content-Type: application/json' -d '{"bizKey":"demo-1","context":{}}'
curl http://localhost:8080/api/v1/healthz
```

## 验证

```bash
gofmt -w $(find . -name '*.go')
go test ./...
go vet ./...
```
