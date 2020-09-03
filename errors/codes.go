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
	ErrLocal       = "701"
	ErrViper       = "702"

	// Mesh specific codes
	// Range 1000-1500
	ErrInstallMesh  = "1001"
	ErrMeshConfig   = "1002"
	ErrPortForward  = "1003"
	ErrClientConfig = "1004"
	ErrClientSet    = "1005"
	ErrStreamEvent  = "1006"
	ErrOpInvalid    = "1007"
	ErrInstallSmi   = "1008"
	ErrConnectSmi   = "1009"
	ErrRunSmi       = "1010"
	ErrDeleteSmi    = "1011"
	ErrSmiInit      = "1012"
)
