module tkestack.io/tapp

// tapp version v1.2.1
go 1.16

require (
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
)

replace tkestack.io/tapp => ../tapp
