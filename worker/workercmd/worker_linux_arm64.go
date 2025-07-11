package workercmd

// The Guardian runtime is not available for linux/arm64
// See: https://github.com/cloudfoundry/garden-runc-release/issues/378
type RuntimeConfiguration struct {
	Runtime string `long:"runtime" default:"containerd" choice:"containerd" choice:"houdini" description:"Runtime to use with the worker. Guardian is not available for linux/arm64. Please note that Houdini is insecure and doesn't run 'tasks' in containers."`
}
