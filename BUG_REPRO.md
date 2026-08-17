# Bug Reproduction

## 包的性质

当前 test_model_fix 保存的是被测模型修复后的结果源码，不是初始含 Bug 源码。要复现原始缺陷，必须检出下面固定的 parent SHA；不要在当前修复结果源码上期待重新出现修复前失败。生成系统使用的可信验证补丁和完整验证日志仅在本地留存，不提交到结果分支。

## 问题现象

在输入站点的首个投产年份恰好等于 subsidy_deadline_year 时，模拟输出把补贴记为 0，并在 subsidy_comparison.csv 中标记为未在窗口内获得。截止年份本身应仍属于可申请窗口。请修复并确保截止年前、截止年和截止年后的行为一致。

## 含 Bug 版本

- 仓库：zhanglei10281852-gif/go-annot-260817-b15-52ed361b-06
- 仓库地址：https://github.com/zhanglei10281852-gif/go-annot-260817-b15-52ed361b-06.git
- parent SHA：f404303a2ab795cfef6a7fde8261d8eeb9b3316f

## 复现步骤

```bash
git clone -- https://github.com/zhanglei10281852-gif/go-annot-260817-b15-52ed361b-06.git bug-repro
cd bug-repro
git checkout --detach f404303a2ab795cfef6a7fde8261d8eeb9b3316f
go test -timeout=120s -count=1 ./internal/monte -run "^TestSubsidyCapturedWhenOperationsStartOnDeadline$"
```

## 双架构完整错误信息

### linux/amd64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test -timeout=120s -count=1 ./internal/monte -run "^TestSubsidyCapturedWhenOperationsStartOnDeadline$"
--- FAIL: TestSubsidyCapturedWhenOperationsStartOnDeadline (0.00s)
    monte_test.go:106: subsidy should be captured in the deadline year, got 0
FAIL
FAIL	eurobatt/internal/monte	0.004s
FAIL

```

stderr：

```text
warning: internal/monte/monte_test.go has type 100755, expected 100644
warning: internal/monte/monte_test.go has type 100755, expected 100644

```

### linux/arm64

- 容器内复现预期退出码：1
- 容器内复现实际退出码：1

stdout：

```text
$ go test -timeout=120s -count=1 ./internal/monte -run "^TestSubsidyCapturedWhenOperationsStartOnDeadline$"
--- FAIL: TestSubsidyCapturedWhenOperationsStartOnDeadline (0.04s)
    monte_test.go:106: subsidy should be captured in the deadline year, got 0
FAIL
FAIL	eurobatt/internal/monte	0.248s
FAIL

```

stderr：

```text
warning: internal/monte/monte_test.go has type 100755, expected 100644
warning: internal/monte/monte_test.go has type 100755, expected 100644

```

## 通过条件

首个投产年份早于或等于截止年份时应计入补贴，晚于截止年份时应为零；定向测试修复前失败、修复后通过，gold 全量测试通过。
