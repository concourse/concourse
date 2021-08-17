/*
Package baggageclaim is the interface for communicating with a BaggageClaim
volume server.

BaggageClaim is an auxilary service that can be collocated with various
container servers (Garden, Docker, etc.) to let them share directories.
BaggageClaim provides a number of benefits over regular bind mounts:

By bringing everything into a the same Volume model we can compose different
technologies together. For example, a Docker image is a stack of layered
volumes which can have a Concourse build cache layered on top of them.

Volumes can be Copy-on-Write (COW) copies of other volumes. This lets us
download a Docker image once and then let it be used by untrusted jobs without
fear that they'll mutate it in some unexpected way. This same COW strategy can
be applied to any volume that BaggageClaim supports.

BaggageClaim volumes go through a three stage lifecycle of being born,
existing, and then dying. This state model is required as creating large
amounts of data can potentially take a long time to materialize. You are only
able to interact with volumes that are in the middle state.

It's the responsibility of the API consumer to delete child volumes before
parent volumes.

The standard way to construct a client is:

	import "github.com/concourse/concourse/worker/baggageclaim/client"

	bcClient := client.New("http://baggageclaim.example.com:7788")
	bcClient.CreateVolume(...)
*/
package baggageclaim
