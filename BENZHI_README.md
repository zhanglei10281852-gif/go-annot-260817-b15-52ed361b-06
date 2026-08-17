# BENZHI_README

## 项目说明

- 项目：zhanglei10281852-gif/go-annot-260817-b15-52ed361b-06
- 项目用途：eurobatt is a local, reproducible scientific-computing tool for simulating the site-selection cost of European power-battery (cell) capacity investments. Given candidate plant sites (Germany, France, Spain, Italy, ...) with multi-year cost data — industrial electricity price, natural-gas price, hourly labor cost, statutory taxes, local-government subsidies, capacity utilization and construction period — it cleans heterogeneous inputs, standardizes everything to EUR per kWh of cell, runs a fixed-
- Go 工具链：`golang:1.26`
- 前端工具链：无

## 标准构建、运行和测试命令

进入容器后执行：

```bash
# 编译
cd '/app' && GOTOOLCHAIN=local go build ./...

# 启动
cd '/app' && GOTOOLCHAIN=local go run ./cmd/eurobatt

# 测试
cd '/app' && GOTOOLCHAIN=local go test ./...
```

## Docker 构建和进入容器

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-task-211-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-task-211-arm64 linux/arm64
docker run -it benzhi-task-211-amd64:latest
docker run -it --platform linux/arm64 benzhi-task-211-arm64:latest
```

## 题目验证命令

1. 预期退出码 0：`go test -timeout=120s -count=1 ./internal/monte -run "^TestSubsidyCapturedWhenOperationsStartOnDeadline$"`
2. 预期退出码 0：`go test -buildvcs=false -count=1 ./...`
3. 预期退出码 0：`GOTOOLCHAIN=local go build -buildvcs=false ./... && GOTOOLCHAIN=local go vet ./...`

## Bug 复现

Bug 现象、触发步骤和完整错误信息见 `BUG_REPRO.md`。
