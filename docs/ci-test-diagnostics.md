# IAM CI test diagnostics

The mainline CI stores the full verbose `go test ./...` output as the `iam-go-test-log` workflow artifact, even when tests fail.

Use this artifact to identify the exact package, test, and assertion before changing production code. CI must validate the checked-out commit and must never mutate a source branch.
