package runtime

// Package runtime is intended to de-couple Concourse Core from a specific runtime implementation.
// As such, the runtime package will
// 			hold abstractions which are applicable to every runtime implementation
//			abstractions which Concourse Core may depend upon ( Core shouldn't depend on upon specific implementations eg. atc/worker )
