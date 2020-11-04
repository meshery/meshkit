package errors

var (

	// GRPC server specific codes
	// Range 600-699
	ErrPanic        = "600"
	ErrGrpcListener = "601"
	ErrGrpcServer   = "602"

	// Config specific codes
	// Range 700-799
	ErrEmptyConfig = "700"
	ErrInMem       = "701"
	ErrViper       = "702"

	// Tracing specific codes
	// Range 800-899

	// Meshery server specific codes
	// Range 10000-10099

	// Meshery adapter specific codes
	// Range 10100 to 10199
	ErrGetName        = "1000"
	ErrInstallMesh    = "1001"
	ErrMeshConfig     = "1002"
	ErrPortForward    = "1003"
	ErrClientConfig   = "1004"
	ErrClientSet      = "1005"
	ErrStreamEvent    = "1006"
	ErrOpInvalid      = "1007"
	ErrApplyOperation = "1008"
	ErrListOperations = "1009"

	// Meshkit specific codes
	// Range 10200 to 10299
	ErrSmiInit    = "10200"
	ErrInstallSmi = "10201"
	ErrConnectSmi = "10202"
	ErrRunSmi     = "10203"
	ErrDeleteSmi  = "10204"
	ErrUnmarshal  = "10205"
	ErrMarshal    = "10205"
	ErrGetBool    = "10205"

	// Istio Service mesh specific codes
	// Range 11000 to 11099

	// Linkerd Service mesh specific codes
	// Range 11100 to 11199

	// Open Service mesh specific codes
	// Range 11200 to 11299

	// Kuma Service mesh specific codes
	// Range 11300 to 11399

	// Citrix Service mesh specific codes
	// Range 11400 to 11499

	// Network Service mesh specific codes
	// Range 11500 to 11599

	// Consul Service mesh specific codes
	// Range 11600 to 11699

	// Octarine Service mesh specific codes
	// Range 11700 to 11799

	// Nginx Service mesh specific codes
	// Range 11800 to 11899

)
