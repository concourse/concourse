package baggageclaim

import "github.com/tedsuo/rata"

const (
	ListVolumes    = "ListVolumes"
	GetVolume      = "GetVolume"
	CreateVolume   = "CreateVolume"
	DestroyVolume  = "DestroyVolume"
	DestroyVolumes = "DestroyVolumes"

	CreateVolumeAsync       = "CreateVolumeAsync"
	CreateVolumeAsyncCancel = "CreateVolumeAsyncCancel"
	CreateVolumeAsyncCheck  = "CreateVolumeAsyncCheck"

	SetProperty   = "SetProperty"
	GetPrivileged = "GetPrivileged"
	SetPrivileged = "SetPrivileged"
	StreamIn      = "StreamIn"
	StreamOut     = "StreamOut"
	StreamP2pOut  = "StreamP2pOut"

	GetP2pUrl = "GetP2pUrl"
)

var Routes = rata.Routes{
	{Path: "/volumes", Method: "GET", Name: ListVolumes},
	{Path: "/volumes", Method: "POST", Name: CreateVolume},

	{Path: "/volumes-async", Method: "POST", Name: CreateVolumeAsync},
	{Path: "/volumes-async/:handle", Method: "GET", Name: CreateVolumeAsyncCheck},
	{Path: "/volumes-async/:handle", Method: "DELETE", Name: CreateVolumeAsyncCancel},

	{Path: "/volumes/:handle", Method: "GET", Name: GetVolume},
	{Path: "/volumes/:handle/properties/:property", Method: "PUT", Name: SetProperty},
	{Path: "/volumes/:handle/privileged", Method: "GET", Name: GetPrivileged},
	{Path: "/volumes/:handle/privileged", Method: "PUT", Name: SetPrivileged},
	{Path: "/volumes/:handle/stream-in", Method: "PUT", Name: StreamIn},
	{Path: "/volumes/:handle/stream-out", Method: "PUT", Name: StreamOut},
	{Path: "/volumes/:handle/stream-p2p-out", Method: "PUT", Name: StreamP2pOut},
	{Path: "/volumes/destroy", Method: "DELETE", Name: DestroyVolumes},
	{Path: "/volumes/:handle", Method: "DELETE", Name: DestroyVolume},

	{Path: "/p2p-url", Method: "GET", Name: GetP2pUrl},
}
