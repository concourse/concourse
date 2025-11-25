package workercmd

type RuntimeConfiguration struct {
	Runtime string `long:"runtime" default:"containerd" choice:"guardian" choice:"containerd" choice:"houdini" description:"Runtime to use with the worker. Please note that Houdini is insecure and doesn't run 'tasks' in containers."`
}
