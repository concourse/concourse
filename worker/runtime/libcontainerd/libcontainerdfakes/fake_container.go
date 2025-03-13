// Code generated by counterfeiter. DO NOT EDIT.
package libcontainerdfakes

import (
	"context"
	"sync"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	typeurl "github.com/containerd/typeurl/v2"
)

type FakeContainer struct {
	CheckpointStub        func(context.Context, string, ...containerd.CheckpointOpts) (containerd.Image, error)
	checkpointMutex       sync.RWMutex
	checkpointArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 []containerd.CheckpointOpts
	}
	checkpointReturns struct {
		result1 containerd.Image
		result2 error
	}
	checkpointReturnsOnCall map[int]struct {
		result1 containerd.Image
		result2 error
	}
	DeleteStub        func(context.Context, ...containerd.DeleteOpts) error
	deleteMutex       sync.RWMutex
	deleteArgsForCall []struct {
		arg1 context.Context
		arg2 []containerd.DeleteOpts
	}
	deleteReturns struct {
		result1 error
	}
	deleteReturnsOnCall map[int]struct {
		result1 error
	}
	ExtensionsStub        func(context.Context) (map[string]typeurl.Any, error)
	extensionsMutex       sync.RWMutex
	extensionsArgsForCall []struct {
		arg1 context.Context
	}
	extensionsReturns struct {
		result1 map[string]typeurl.Any
		result2 error
	}
	extensionsReturnsOnCall map[int]struct {
		result1 map[string]typeurl.Any
		result2 error
	}
	IDStub        func() string
	iDMutex       sync.RWMutex
	iDArgsForCall []struct {
	}
	iDReturns struct {
		result1 string
	}
	iDReturnsOnCall map[int]struct {
		result1 string
	}
	ImageStub        func(context.Context) (containerd.Image, error)
	imageMutex       sync.RWMutex
	imageArgsForCall []struct {
		arg1 context.Context
	}
	imageReturns struct {
		result1 containerd.Image
		result2 error
	}
	imageReturnsOnCall map[int]struct {
		result1 containerd.Image
		result2 error
	}
	InfoStub        func(context.Context, ...containerd.InfoOpts) (containers.Container, error)
	infoMutex       sync.RWMutex
	infoArgsForCall []struct {
		arg1 context.Context
		arg2 []containerd.InfoOpts
	}
	infoReturns struct {
		result1 containers.Container
		result2 error
	}
	infoReturnsOnCall map[int]struct {
		result1 containers.Container
		result2 error
	}
	LabelsStub        func(context.Context) (map[string]string, error)
	labelsMutex       sync.RWMutex
	labelsArgsForCall []struct {
		arg1 context.Context
	}
	labelsReturns struct {
		result1 map[string]string
		result2 error
	}
	labelsReturnsOnCall map[int]struct {
		result1 map[string]string
		result2 error
	}
	NewTaskStub        func(context.Context, cio.Creator, ...containerd.NewTaskOpts) (containerd.Task, error)
	newTaskMutex       sync.RWMutex
	newTaskArgsForCall []struct {
		arg1 context.Context
		arg2 cio.Creator
		arg3 []containerd.NewTaskOpts
	}
	newTaskReturns struct {
		result1 containerd.Task
		result2 error
	}
	newTaskReturnsOnCall map[int]struct {
		result1 containerd.Task
		result2 error
	}
	SetLabelsStub        func(context.Context, map[string]string) (map[string]string, error)
	setLabelsMutex       sync.RWMutex
	setLabelsArgsForCall []struct {
		arg1 context.Context
		arg2 map[string]string
	}
	setLabelsReturns struct {
		result1 map[string]string
		result2 error
	}
	setLabelsReturnsOnCall map[int]struct {
		result1 map[string]string
		result2 error
	}
	SpecStub        func(context.Context) (*oci.Spec, error)
	specMutex       sync.RWMutex
	specArgsForCall []struct {
		arg1 context.Context
	}
	specReturns struct {
		result1 *oci.Spec
		result2 error
	}
	specReturnsOnCall map[int]struct {
		result1 *oci.Spec
		result2 error
	}
	TaskStub        func(context.Context, cio.Attach) (containerd.Task, error)
	taskMutex       sync.RWMutex
	taskArgsForCall []struct {
		arg1 context.Context
		arg2 cio.Attach
	}
	taskReturns struct {
		result1 containerd.Task
		result2 error
	}
	taskReturnsOnCall map[int]struct {
		result1 containerd.Task
		result2 error
	}
	UpdateStub        func(context.Context, ...containerd.UpdateContainerOpts) error
	updateMutex       sync.RWMutex
	updateArgsForCall []struct {
		arg1 context.Context
		arg2 []containerd.UpdateContainerOpts
	}
	updateReturns struct {
		result1 error
	}
	updateReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]any
	invocationsMutex sync.RWMutex
}

func (fake *FakeContainer) Checkpoint(arg1 context.Context, arg2 string, arg3 ...containerd.CheckpointOpts) (containerd.Image, error) {
	fake.checkpointMutex.Lock()
	ret, specificReturn := fake.checkpointReturnsOnCall[len(fake.checkpointArgsForCall)]
	fake.checkpointArgsForCall = append(fake.checkpointArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 []containerd.CheckpointOpts
	}{arg1, arg2, arg3})
	stub := fake.CheckpointStub
	fakeReturns := fake.checkpointReturns
	fake.recordInvocation("Checkpoint", []any{arg1, arg2, arg3})
	fake.checkpointMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeContainer) CheckpointCallCount() int {
	fake.checkpointMutex.RLock()
	defer fake.checkpointMutex.RUnlock()
	return len(fake.checkpointArgsForCall)
}

func (fake *FakeContainer) CheckpointCalls(stub func(context.Context, string, ...containerd.CheckpointOpts) (containerd.Image, error)) {
	fake.checkpointMutex.Lock()
	defer fake.checkpointMutex.Unlock()
	fake.CheckpointStub = stub
}

func (fake *FakeContainer) CheckpointArgsForCall(i int) (context.Context, string, []containerd.CheckpointOpts) {
	fake.checkpointMutex.RLock()
	defer fake.checkpointMutex.RUnlock()
	argsForCall := fake.checkpointArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeContainer) CheckpointReturns(result1 containerd.Image, result2 error) {
	fake.checkpointMutex.Lock()
	defer fake.checkpointMutex.Unlock()
	fake.CheckpointStub = nil
	fake.checkpointReturns = struct {
		result1 containerd.Image
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) CheckpointReturnsOnCall(i int, result1 containerd.Image, result2 error) {
	fake.checkpointMutex.Lock()
	defer fake.checkpointMutex.Unlock()
	fake.CheckpointStub = nil
	if fake.checkpointReturnsOnCall == nil {
		fake.checkpointReturnsOnCall = make(map[int]struct {
			result1 containerd.Image
			result2 error
		})
	}
	fake.checkpointReturnsOnCall[i] = struct {
		result1 containerd.Image
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) Delete(arg1 context.Context, arg2 ...containerd.DeleteOpts) error {
	fake.deleteMutex.Lock()
	ret, specificReturn := fake.deleteReturnsOnCall[len(fake.deleteArgsForCall)]
	fake.deleteArgsForCall = append(fake.deleteArgsForCall, struct {
		arg1 context.Context
		arg2 []containerd.DeleteOpts
	}{arg1, arg2})
	stub := fake.DeleteStub
	fakeReturns := fake.deleteReturns
	fake.recordInvocation("Delete", []any{arg1, arg2})
	fake.deleteMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeContainer) DeleteCallCount() int {
	fake.deleteMutex.RLock()
	defer fake.deleteMutex.RUnlock()
	return len(fake.deleteArgsForCall)
}

func (fake *FakeContainer) DeleteCalls(stub func(context.Context, ...containerd.DeleteOpts) error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = stub
}

func (fake *FakeContainer) DeleteArgsForCall(i int) (context.Context, []containerd.DeleteOpts) {
	fake.deleteMutex.RLock()
	defer fake.deleteMutex.RUnlock()
	argsForCall := fake.deleteArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeContainer) DeleteReturns(result1 error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = nil
	fake.deleteReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeContainer) DeleteReturnsOnCall(i int, result1 error) {
	fake.deleteMutex.Lock()
	defer fake.deleteMutex.Unlock()
	fake.DeleteStub = nil
	if fake.deleteReturnsOnCall == nil {
		fake.deleteReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.deleteReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeContainer) Extensions(arg1 context.Context) (map[string]typeurl.Any, error) {
	fake.extensionsMutex.Lock()
	ret, specificReturn := fake.extensionsReturnsOnCall[len(fake.extensionsArgsForCall)]
	fake.extensionsArgsForCall = append(fake.extensionsArgsForCall, struct {
		arg1 context.Context
	}{arg1})
	stub := fake.ExtensionsStub
	fakeReturns := fake.extensionsReturns
	fake.recordInvocation("Extensions", []any{arg1})
	fake.extensionsMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeContainer) ExtensionsCallCount() int {
	fake.extensionsMutex.RLock()
	defer fake.extensionsMutex.RUnlock()
	return len(fake.extensionsArgsForCall)
}

func (fake *FakeContainer) ExtensionsCalls(stub func(context.Context) (map[string]typeurl.Any, error)) {
	fake.extensionsMutex.Lock()
	defer fake.extensionsMutex.Unlock()
	fake.ExtensionsStub = stub
}

func (fake *FakeContainer) ExtensionsArgsForCall(i int) context.Context {
	fake.extensionsMutex.RLock()
	defer fake.extensionsMutex.RUnlock()
	argsForCall := fake.extensionsArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeContainer) ExtensionsReturns(result1 map[string]typeurl.Any, result2 error) {
	fake.extensionsMutex.Lock()
	defer fake.extensionsMutex.Unlock()
	fake.ExtensionsStub = nil
	fake.extensionsReturns = struct {
		result1 map[string]typeurl.Any
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) ExtensionsReturnsOnCall(i int, result1 map[string]typeurl.Any, result2 error) {
	fake.extensionsMutex.Lock()
	defer fake.extensionsMutex.Unlock()
	fake.ExtensionsStub = nil
	if fake.extensionsReturnsOnCall == nil {
		fake.extensionsReturnsOnCall = make(map[int]struct {
			result1 map[string]typeurl.Any
			result2 error
		})
	}
	fake.extensionsReturnsOnCall[i] = struct {
		result1 map[string]typeurl.Any
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) ID() string {
	fake.iDMutex.Lock()
	ret, specificReturn := fake.iDReturnsOnCall[len(fake.iDArgsForCall)]
	fake.iDArgsForCall = append(fake.iDArgsForCall, struct {
	}{})
	stub := fake.IDStub
	fakeReturns := fake.iDReturns
	fake.recordInvocation("ID", []any{})
	fake.iDMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeContainer) IDCallCount() int {
	fake.iDMutex.RLock()
	defer fake.iDMutex.RUnlock()
	return len(fake.iDArgsForCall)
}

func (fake *FakeContainer) IDCalls(stub func() string) {
	fake.iDMutex.Lock()
	defer fake.iDMutex.Unlock()
	fake.IDStub = stub
}

func (fake *FakeContainer) IDReturns(result1 string) {
	fake.iDMutex.Lock()
	defer fake.iDMutex.Unlock()
	fake.IDStub = nil
	fake.iDReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeContainer) IDReturnsOnCall(i int, result1 string) {
	fake.iDMutex.Lock()
	defer fake.iDMutex.Unlock()
	fake.IDStub = nil
	if fake.iDReturnsOnCall == nil {
		fake.iDReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.iDReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeContainer) Image(arg1 context.Context) (containerd.Image, error) {
	fake.imageMutex.Lock()
	ret, specificReturn := fake.imageReturnsOnCall[len(fake.imageArgsForCall)]
	fake.imageArgsForCall = append(fake.imageArgsForCall, struct {
		arg1 context.Context
	}{arg1})
	stub := fake.ImageStub
	fakeReturns := fake.imageReturns
	fake.recordInvocation("Image", []any{arg1})
	fake.imageMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeContainer) ImageCallCount() int {
	fake.imageMutex.RLock()
	defer fake.imageMutex.RUnlock()
	return len(fake.imageArgsForCall)
}

func (fake *FakeContainer) ImageCalls(stub func(context.Context) (containerd.Image, error)) {
	fake.imageMutex.Lock()
	defer fake.imageMutex.Unlock()
	fake.ImageStub = stub
}

func (fake *FakeContainer) ImageArgsForCall(i int) context.Context {
	fake.imageMutex.RLock()
	defer fake.imageMutex.RUnlock()
	argsForCall := fake.imageArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeContainer) ImageReturns(result1 containerd.Image, result2 error) {
	fake.imageMutex.Lock()
	defer fake.imageMutex.Unlock()
	fake.ImageStub = nil
	fake.imageReturns = struct {
		result1 containerd.Image
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) ImageReturnsOnCall(i int, result1 containerd.Image, result2 error) {
	fake.imageMutex.Lock()
	defer fake.imageMutex.Unlock()
	fake.ImageStub = nil
	if fake.imageReturnsOnCall == nil {
		fake.imageReturnsOnCall = make(map[int]struct {
			result1 containerd.Image
			result2 error
		})
	}
	fake.imageReturnsOnCall[i] = struct {
		result1 containerd.Image
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) Info(arg1 context.Context, arg2 ...containerd.InfoOpts) (containers.Container, error) {
	fake.infoMutex.Lock()
	ret, specificReturn := fake.infoReturnsOnCall[len(fake.infoArgsForCall)]
	fake.infoArgsForCall = append(fake.infoArgsForCall, struct {
		arg1 context.Context
		arg2 []containerd.InfoOpts
	}{arg1, arg2})
	stub := fake.InfoStub
	fakeReturns := fake.infoReturns
	fake.recordInvocation("Info", []any{arg1, arg2})
	fake.infoMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2...)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeContainer) InfoCallCount() int {
	fake.infoMutex.RLock()
	defer fake.infoMutex.RUnlock()
	return len(fake.infoArgsForCall)
}

func (fake *FakeContainer) InfoCalls(stub func(context.Context, ...containerd.InfoOpts) (containers.Container, error)) {
	fake.infoMutex.Lock()
	defer fake.infoMutex.Unlock()
	fake.InfoStub = stub
}

func (fake *FakeContainer) InfoArgsForCall(i int) (context.Context, []containerd.InfoOpts) {
	fake.infoMutex.RLock()
	defer fake.infoMutex.RUnlock()
	argsForCall := fake.infoArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeContainer) InfoReturns(result1 containers.Container, result2 error) {
	fake.infoMutex.Lock()
	defer fake.infoMutex.Unlock()
	fake.InfoStub = nil
	fake.infoReturns = struct {
		result1 containers.Container
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) InfoReturnsOnCall(i int, result1 containers.Container, result2 error) {
	fake.infoMutex.Lock()
	defer fake.infoMutex.Unlock()
	fake.InfoStub = nil
	if fake.infoReturnsOnCall == nil {
		fake.infoReturnsOnCall = make(map[int]struct {
			result1 containers.Container
			result2 error
		})
	}
	fake.infoReturnsOnCall[i] = struct {
		result1 containers.Container
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) Labels(arg1 context.Context) (map[string]string, error) {
	fake.labelsMutex.Lock()
	ret, specificReturn := fake.labelsReturnsOnCall[len(fake.labelsArgsForCall)]
	fake.labelsArgsForCall = append(fake.labelsArgsForCall, struct {
		arg1 context.Context
	}{arg1})
	stub := fake.LabelsStub
	fakeReturns := fake.labelsReturns
	fake.recordInvocation("Labels", []any{arg1})
	fake.labelsMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeContainer) LabelsCallCount() int {
	fake.labelsMutex.RLock()
	defer fake.labelsMutex.RUnlock()
	return len(fake.labelsArgsForCall)
}

func (fake *FakeContainer) LabelsCalls(stub func(context.Context) (map[string]string, error)) {
	fake.labelsMutex.Lock()
	defer fake.labelsMutex.Unlock()
	fake.LabelsStub = stub
}

func (fake *FakeContainer) LabelsArgsForCall(i int) context.Context {
	fake.labelsMutex.RLock()
	defer fake.labelsMutex.RUnlock()
	argsForCall := fake.labelsArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeContainer) LabelsReturns(result1 map[string]string, result2 error) {
	fake.labelsMutex.Lock()
	defer fake.labelsMutex.Unlock()
	fake.LabelsStub = nil
	fake.labelsReturns = struct {
		result1 map[string]string
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) LabelsReturnsOnCall(i int, result1 map[string]string, result2 error) {
	fake.labelsMutex.Lock()
	defer fake.labelsMutex.Unlock()
	fake.LabelsStub = nil
	if fake.labelsReturnsOnCall == nil {
		fake.labelsReturnsOnCall = make(map[int]struct {
			result1 map[string]string
			result2 error
		})
	}
	fake.labelsReturnsOnCall[i] = struct {
		result1 map[string]string
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) NewTask(arg1 context.Context, arg2 cio.Creator, arg3 ...containerd.NewTaskOpts) (containerd.Task, error) {
	fake.newTaskMutex.Lock()
	ret, specificReturn := fake.newTaskReturnsOnCall[len(fake.newTaskArgsForCall)]
	fake.newTaskArgsForCall = append(fake.newTaskArgsForCall, struct {
		arg1 context.Context
		arg2 cio.Creator
		arg3 []containerd.NewTaskOpts
	}{arg1, arg2, arg3})
	stub := fake.NewTaskStub
	fakeReturns := fake.newTaskReturns
	fake.recordInvocation("NewTask", []any{arg1, arg2, arg3})
	fake.newTaskMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeContainer) NewTaskCallCount() int {
	fake.newTaskMutex.RLock()
	defer fake.newTaskMutex.RUnlock()
	return len(fake.newTaskArgsForCall)
}

func (fake *FakeContainer) NewTaskCalls(stub func(context.Context, cio.Creator, ...containerd.NewTaskOpts) (containerd.Task, error)) {
	fake.newTaskMutex.Lock()
	defer fake.newTaskMutex.Unlock()
	fake.NewTaskStub = stub
}

func (fake *FakeContainer) NewTaskArgsForCall(i int) (context.Context, cio.Creator, []containerd.NewTaskOpts) {
	fake.newTaskMutex.RLock()
	defer fake.newTaskMutex.RUnlock()
	argsForCall := fake.newTaskArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeContainer) NewTaskReturns(result1 containerd.Task, result2 error) {
	fake.newTaskMutex.Lock()
	defer fake.newTaskMutex.Unlock()
	fake.NewTaskStub = nil
	fake.newTaskReturns = struct {
		result1 containerd.Task
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) NewTaskReturnsOnCall(i int, result1 containerd.Task, result2 error) {
	fake.newTaskMutex.Lock()
	defer fake.newTaskMutex.Unlock()
	fake.NewTaskStub = nil
	if fake.newTaskReturnsOnCall == nil {
		fake.newTaskReturnsOnCall = make(map[int]struct {
			result1 containerd.Task
			result2 error
		})
	}
	fake.newTaskReturnsOnCall[i] = struct {
		result1 containerd.Task
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) SetLabels(arg1 context.Context, arg2 map[string]string) (map[string]string, error) {
	fake.setLabelsMutex.Lock()
	ret, specificReturn := fake.setLabelsReturnsOnCall[len(fake.setLabelsArgsForCall)]
	fake.setLabelsArgsForCall = append(fake.setLabelsArgsForCall, struct {
		arg1 context.Context
		arg2 map[string]string
	}{arg1, arg2})
	stub := fake.SetLabelsStub
	fakeReturns := fake.setLabelsReturns
	fake.recordInvocation("SetLabels", []any{arg1, arg2})
	fake.setLabelsMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeContainer) SetLabelsCallCount() int {
	fake.setLabelsMutex.RLock()
	defer fake.setLabelsMutex.RUnlock()
	return len(fake.setLabelsArgsForCall)
}

func (fake *FakeContainer) SetLabelsCalls(stub func(context.Context, map[string]string) (map[string]string, error)) {
	fake.setLabelsMutex.Lock()
	defer fake.setLabelsMutex.Unlock()
	fake.SetLabelsStub = stub
}

func (fake *FakeContainer) SetLabelsArgsForCall(i int) (context.Context, map[string]string) {
	fake.setLabelsMutex.RLock()
	defer fake.setLabelsMutex.RUnlock()
	argsForCall := fake.setLabelsArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeContainer) SetLabelsReturns(result1 map[string]string, result2 error) {
	fake.setLabelsMutex.Lock()
	defer fake.setLabelsMutex.Unlock()
	fake.SetLabelsStub = nil
	fake.setLabelsReturns = struct {
		result1 map[string]string
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) SetLabelsReturnsOnCall(i int, result1 map[string]string, result2 error) {
	fake.setLabelsMutex.Lock()
	defer fake.setLabelsMutex.Unlock()
	fake.SetLabelsStub = nil
	if fake.setLabelsReturnsOnCall == nil {
		fake.setLabelsReturnsOnCall = make(map[int]struct {
			result1 map[string]string
			result2 error
		})
	}
	fake.setLabelsReturnsOnCall[i] = struct {
		result1 map[string]string
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) Spec(arg1 context.Context) (*oci.Spec, error) {
	fake.specMutex.Lock()
	ret, specificReturn := fake.specReturnsOnCall[len(fake.specArgsForCall)]
	fake.specArgsForCall = append(fake.specArgsForCall, struct {
		arg1 context.Context
	}{arg1})
	stub := fake.SpecStub
	fakeReturns := fake.specReturns
	fake.recordInvocation("Spec", []any{arg1})
	fake.specMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeContainer) SpecCallCount() int {
	fake.specMutex.RLock()
	defer fake.specMutex.RUnlock()
	return len(fake.specArgsForCall)
}

func (fake *FakeContainer) SpecCalls(stub func(context.Context) (*oci.Spec, error)) {
	fake.specMutex.Lock()
	defer fake.specMutex.Unlock()
	fake.SpecStub = stub
}

func (fake *FakeContainer) SpecArgsForCall(i int) context.Context {
	fake.specMutex.RLock()
	defer fake.specMutex.RUnlock()
	argsForCall := fake.specArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeContainer) SpecReturns(result1 *oci.Spec, result2 error) {
	fake.specMutex.Lock()
	defer fake.specMutex.Unlock()
	fake.SpecStub = nil
	fake.specReturns = struct {
		result1 *oci.Spec
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) SpecReturnsOnCall(i int, result1 *oci.Spec, result2 error) {
	fake.specMutex.Lock()
	defer fake.specMutex.Unlock()
	fake.SpecStub = nil
	if fake.specReturnsOnCall == nil {
		fake.specReturnsOnCall = make(map[int]struct {
			result1 *oci.Spec
			result2 error
		})
	}
	fake.specReturnsOnCall[i] = struct {
		result1 *oci.Spec
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) Task(arg1 context.Context, arg2 cio.Attach) (containerd.Task, error) {
	fake.taskMutex.Lock()
	ret, specificReturn := fake.taskReturnsOnCall[len(fake.taskArgsForCall)]
	fake.taskArgsForCall = append(fake.taskArgsForCall, struct {
		arg1 context.Context
		arg2 cio.Attach
	}{arg1, arg2})
	stub := fake.TaskStub
	fakeReturns := fake.taskReturns
	fake.recordInvocation("Task", []any{arg1, arg2})
	fake.taskMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeContainer) TaskCallCount() int {
	fake.taskMutex.RLock()
	defer fake.taskMutex.RUnlock()
	return len(fake.taskArgsForCall)
}

func (fake *FakeContainer) TaskCalls(stub func(context.Context, cio.Attach) (containerd.Task, error)) {
	fake.taskMutex.Lock()
	defer fake.taskMutex.Unlock()
	fake.TaskStub = stub
}

func (fake *FakeContainer) TaskArgsForCall(i int) (context.Context, cio.Attach) {
	fake.taskMutex.RLock()
	defer fake.taskMutex.RUnlock()
	argsForCall := fake.taskArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeContainer) TaskReturns(result1 containerd.Task, result2 error) {
	fake.taskMutex.Lock()
	defer fake.taskMutex.Unlock()
	fake.TaskStub = nil
	fake.taskReturns = struct {
		result1 containerd.Task
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) TaskReturnsOnCall(i int, result1 containerd.Task, result2 error) {
	fake.taskMutex.Lock()
	defer fake.taskMutex.Unlock()
	fake.TaskStub = nil
	if fake.taskReturnsOnCall == nil {
		fake.taskReturnsOnCall = make(map[int]struct {
			result1 containerd.Task
			result2 error
		})
	}
	fake.taskReturnsOnCall[i] = struct {
		result1 containerd.Task
		result2 error
	}{result1, result2}
}

func (fake *FakeContainer) Update(arg1 context.Context, arg2 ...containerd.UpdateContainerOpts) error {
	fake.updateMutex.Lock()
	ret, specificReturn := fake.updateReturnsOnCall[len(fake.updateArgsForCall)]
	fake.updateArgsForCall = append(fake.updateArgsForCall, struct {
		arg1 context.Context
		arg2 []containerd.UpdateContainerOpts
	}{arg1, arg2})
	stub := fake.UpdateStub
	fakeReturns := fake.updateReturns
	fake.recordInvocation("Update", []any{arg1, arg2})
	fake.updateMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeContainer) UpdateCallCount() int {
	fake.updateMutex.RLock()
	defer fake.updateMutex.RUnlock()
	return len(fake.updateArgsForCall)
}

func (fake *FakeContainer) UpdateCalls(stub func(context.Context, ...containerd.UpdateContainerOpts) error) {
	fake.updateMutex.Lock()
	defer fake.updateMutex.Unlock()
	fake.UpdateStub = stub
}

func (fake *FakeContainer) UpdateArgsForCall(i int) (context.Context, []containerd.UpdateContainerOpts) {
	fake.updateMutex.RLock()
	defer fake.updateMutex.RUnlock()
	argsForCall := fake.updateArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeContainer) UpdateReturns(result1 error) {
	fake.updateMutex.Lock()
	defer fake.updateMutex.Unlock()
	fake.UpdateStub = nil
	fake.updateReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeContainer) UpdateReturnsOnCall(i int, result1 error) {
	fake.updateMutex.Lock()
	defer fake.updateMutex.Unlock()
	fake.UpdateStub = nil
	if fake.updateReturnsOnCall == nil {
		fake.updateReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.updateReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeContainer) Invocations() map[string][][]any {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.checkpointMutex.RLock()
	defer fake.checkpointMutex.RUnlock()
	fake.deleteMutex.RLock()
	defer fake.deleteMutex.RUnlock()
	fake.extensionsMutex.RLock()
	defer fake.extensionsMutex.RUnlock()
	fake.iDMutex.RLock()
	defer fake.iDMutex.RUnlock()
	fake.imageMutex.RLock()
	defer fake.imageMutex.RUnlock()
	fake.infoMutex.RLock()
	defer fake.infoMutex.RUnlock()
	fake.labelsMutex.RLock()
	defer fake.labelsMutex.RUnlock()
	fake.newTaskMutex.RLock()
	defer fake.newTaskMutex.RUnlock()
	fake.setLabelsMutex.RLock()
	defer fake.setLabelsMutex.RUnlock()
	fake.specMutex.RLock()
	defer fake.specMutex.RUnlock()
	fake.taskMutex.RLock()
	defer fake.taskMutex.RUnlock()
	fake.updateMutex.RLock()
	defer fake.updateMutex.RUnlock()
	copiedInvocations := map[string][][]any{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeContainer) recordInvocation(key string, args []any) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]any{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]any{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ containerd.Container = new(FakeContainer)
