module github.com/ItsThompson/gofin/services/reporting

go 1.26

require (
	github.com/ItsThompson/gofin/services/auth v0.0.0
	github.com/ItsThompson/gofin/services/datarights v0.0.0
	github.com/ItsThompson/gofin/services/expense v0.0.0
	github.com/ItsThompson/gofin/services/finance v0.0.0
	github.com/ItsThompson/gofin/services/shared/reporting v0.0.0
	google.golang.org/grpc v1.80.0
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260401024825-9d38bb4040a9 // indirect
)

replace github.com/ItsThompson/gofin/services/auth => ../auth

replace github.com/ItsThompson/gofin/services/datarights => ../datarights

replace github.com/ItsThompson/gofin/services/expense => ../expense

replace github.com/ItsThompson/gofin/services/finance => ../finance

replace github.com/ItsThompson/gofin/services/shared/reporting => ../shared/reporting
