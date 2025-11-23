module github.com/absfs/metricsfs/examples/basic

go 1.23.0

replace github.com/absfs/metricsfs => ../..

require (
	github.com/absfs/metricsfs v0.0.0-00010101000000-000000000000
	github.com/absfs/osfs v0.0.0-20220705103527-80b6215cf130
	github.com/prometheus/client_golang v1.23.2
)
