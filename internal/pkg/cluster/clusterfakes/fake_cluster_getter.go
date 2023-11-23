// Code generated by counterfeiter. DO NOT EDIT.
package clusterfakes

import (
	"context"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
	"github.com/giantswarm/object-storage-operator/internal/pkg/flags"
)

type FakeClusterGetter struct {
	GetClusterStub        func(context.Context, client.Client, flags.ManagementCluster) (cluster.Cluster, error)
	getClusterMutex       sync.RWMutex
	getClusterArgsForCall []struct {
		arg1 context.Context
		arg2 client.Client
		arg3 flags.ManagementCluster
	}
	getClusterReturns struct {
		result1 cluster.Cluster
		result2 error
	}
	getClusterReturnsOnCall map[int]struct {
		result1 cluster.Cluster
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeClusterGetter) GetCluster(arg1 context.Context, arg2 client.Client, arg3 flags.ManagementCluster) (cluster.Cluster, error) {
	fake.getClusterMutex.Lock()
	ret, specificReturn := fake.getClusterReturnsOnCall[len(fake.getClusterArgsForCall)]
	fake.getClusterArgsForCall = append(fake.getClusterArgsForCall, struct {
		arg1 context.Context
		arg2 client.Client
		arg3 flags.ManagementCluster
	}{arg1, arg2, arg3})
	stub := fake.GetClusterStub
	fakeReturns := fake.getClusterReturns
	fake.recordInvocation("GetCluster", []interface{}{arg1, arg2, arg3})
	fake.getClusterMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeClusterGetter) GetClusterCallCount() int {
	fake.getClusterMutex.RLock()
	defer fake.getClusterMutex.RUnlock()
	return len(fake.getClusterArgsForCall)
}

func (fake *FakeClusterGetter) GetClusterCalls(stub func(context.Context, client.Client, flags.ManagementCluster) (cluster.Cluster, error)) {
	fake.getClusterMutex.Lock()
	defer fake.getClusterMutex.Unlock()
	fake.GetClusterStub = stub
}

func (fake *FakeClusterGetter) GetClusterArgsForCall(i int) (context.Context, client.Client, flags.ManagementCluster) {
	fake.getClusterMutex.RLock()
	defer fake.getClusterMutex.RUnlock()
	argsForCall := fake.getClusterArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeClusterGetter) GetClusterReturns(result1 cluster.Cluster, result2 error) {
	fake.getClusterMutex.Lock()
	defer fake.getClusterMutex.Unlock()
	fake.GetClusterStub = nil
	fake.getClusterReturns = struct {
		result1 cluster.Cluster
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterGetter) GetClusterReturnsOnCall(i int, result1 cluster.Cluster, result2 error) {
	fake.getClusterMutex.Lock()
	defer fake.getClusterMutex.Unlock()
	fake.GetClusterStub = nil
	if fake.getClusterReturnsOnCall == nil {
		fake.getClusterReturnsOnCall = make(map[int]struct {
			result1 cluster.Cluster
			result2 error
		})
	}
	fake.getClusterReturnsOnCall[i] = struct {
		result1 cluster.Cluster
		result2 error
	}{result1, result2}
}

func (fake *FakeClusterGetter) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getClusterMutex.RLock()
	defer fake.getClusterMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeClusterGetter) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ cluster.ClusterGetter = new(FakeClusterGetter)
