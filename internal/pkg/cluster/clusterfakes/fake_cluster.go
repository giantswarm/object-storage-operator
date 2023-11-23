// Code generated by counterfeiter. DO NOT EDIT.
package clusterfakes

import (
	"sync"

	"github.com/giantswarm/object-storage-operator/internal/pkg/cluster"
)

type FakeCluster struct {
	GetBaseDomainStub        func() string
	getBaseDomainMutex       sync.RWMutex
	getBaseDomainArgsForCall []struct {
	}
	getBaseDomainReturns struct {
		result1 string
	}
	getBaseDomainReturnsOnCall map[int]struct {
		result1 string
	}
	GetNameStub        func() string
	getNameMutex       sync.RWMutex
	getNameArgsForCall []struct {
	}
	getNameReturns struct {
		result1 string
	}
	getNameReturnsOnCall map[int]struct {
		result1 string
	}
	GetNamespaceStub        func() string
	getNamespaceMutex       sync.RWMutex
	getNamespaceArgsForCall []struct {
	}
	getNamespaceReturns struct {
		result1 string
	}
	getNamespaceReturnsOnCall map[int]struct {
		result1 string
	}
	GetRegionStub        func() string
	getRegionMutex       sync.RWMutex
	getRegionArgsForCall []struct {
	}
	getRegionReturns struct {
		result1 string
	}
	getRegionReturnsOnCall map[int]struct {
		result1 string
	}
	GetRoleStub        func() string
	getRoleMutex       sync.RWMutex
	getRoleArgsForCall []struct {
	}
	getRoleReturns struct {
		result1 string
	}
	getRoleReturnsOnCall map[int]struct {
		result1 string
	}
	GetTagsStub        func() map[string]string
	getTagsMutex       sync.RWMutex
	getTagsArgsForCall []struct {
	}
	getTagsReturns struct {
		result1 map[string]string
	}
	getTagsReturnsOnCall map[int]struct {
		result1 map[string]string
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeCluster) GetBaseDomain() string {
	fake.getBaseDomainMutex.Lock()
	ret, specificReturn := fake.getBaseDomainReturnsOnCall[len(fake.getBaseDomainArgsForCall)]
	fake.getBaseDomainArgsForCall = append(fake.getBaseDomainArgsForCall, struct {
	}{})
	stub := fake.GetBaseDomainStub
	fakeReturns := fake.getBaseDomainReturns
	fake.recordInvocation("GetBaseDomain", []interface{}{})
	fake.getBaseDomainMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeCluster) GetBaseDomainCallCount() int {
	fake.getBaseDomainMutex.RLock()
	defer fake.getBaseDomainMutex.RUnlock()
	return len(fake.getBaseDomainArgsForCall)
}

func (fake *FakeCluster) GetBaseDomainCalls(stub func() string) {
	fake.getBaseDomainMutex.Lock()
	defer fake.getBaseDomainMutex.Unlock()
	fake.GetBaseDomainStub = stub
}

func (fake *FakeCluster) GetBaseDomainReturns(result1 string) {
	fake.getBaseDomainMutex.Lock()
	defer fake.getBaseDomainMutex.Unlock()
	fake.GetBaseDomainStub = nil
	fake.getBaseDomainReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeCluster) GetBaseDomainReturnsOnCall(i int, result1 string) {
	fake.getBaseDomainMutex.Lock()
	defer fake.getBaseDomainMutex.Unlock()
	fake.GetBaseDomainStub = nil
	if fake.getBaseDomainReturnsOnCall == nil {
		fake.getBaseDomainReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.getBaseDomainReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeCluster) GetName() string {
	fake.getNameMutex.Lock()
	ret, specificReturn := fake.getNameReturnsOnCall[len(fake.getNameArgsForCall)]
	fake.getNameArgsForCall = append(fake.getNameArgsForCall, struct {
	}{})
	stub := fake.GetNameStub
	fakeReturns := fake.getNameReturns
	fake.recordInvocation("GetName", []interface{}{})
	fake.getNameMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeCluster) GetNameCallCount() int {
	fake.getNameMutex.RLock()
	defer fake.getNameMutex.RUnlock()
	return len(fake.getNameArgsForCall)
}

func (fake *FakeCluster) GetNameCalls(stub func() string) {
	fake.getNameMutex.Lock()
	defer fake.getNameMutex.Unlock()
	fake.GetNameStub = stub
}

func (fake *FakeCluster) GetNameReturns(result1 string) {
	fake.getNameMutex.Lock()
	defer fake.getNameMutex.Unlock()
	fake.GetNameStub = nil
	fake.getNameReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeCluster) GetNameReturnsOnCall(i int, result1 string) {
	fake.getNameMutex.Lock()
	defer fake.getNameMutex.Unlock()
	fake.GetNameStub = nil
	if fake.getNameReturnsOnCall == nil {
		fake.getNameReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.getNameReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeCluster) GetNamespace() string {
	fake.getNamespaceMutex.Lock()
	ret, specificReturn := fake.getNamespaceReturnsOnCall[len(fake.getNamespaceArgsForCall)]
	fake.getNamespaceArgsForCall = append(fake.getNamespaceArgsForCall, struct {
	}{})
	stub := fake.GetNamespaceStub
	fakeReturns := fake.getNamespaceReturns
	fake.recordInvocation("GetNamespace", []interface{}{})
	fake.getNamespaceMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeCluster) GetNamespaceCallCount() int {
	fake.getNamespaceMutex.RLock()
	defer fake.getNamespaceMutex.RUnlock()
	return len(fake.getNamespaceArgsForCall)
}

func (fake *FakeCluster) GetNamespaceCalls(stub func() string) {
	fake.getNamespaceMutex.Lock()
	defer fake.getNamespaceMutex.Unlock()
	fake.GetNamespaceStub = stub
}

func (fake *FakeCluster) GetNamespaceReturns(result1 string) {
	fake.getNamespaceMutex.Lock()
	defer fake.getNamespaceMutex.Unlock()
	fake.GetNamespaceStub = nil
	fake.getNamespaceReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeCluster) GetNamespaceReturnsOnCall(i int, result1 string) {
	fake.getNamespaceMutex.Lock()
	defer fake.getNamespaceMutex.Unlock()
	fake.GetNamespaceStub = nil
	if fake.getNamespaceReturnsOnCall == nil {
		fake.getNamespaceReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.getNamespaceReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeCluster) GetRegion() string {
	fake.getRegionMutex.Lock()
	ret, specificReturn := fake.getRegionReturnsOnCall[len(fake.getRegionArgsForCall)]
	fake.getRegionArgsForCall = append(fake.getRegionArgsForCall, struct {
	}{})
	stub := fake.GetRegionStub
	fakeReturns := fake.getRegionReturns
	fake.recordInvocation("GetRegion", []interface{}{})
	fake.getRegionMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeCluster) GetRegionCallCount() int {
	fake.getRegionMutex.RLock()
	defer fake.getRegionMutex.RUnlock()
	return len(fake.getRegionArgsForCall)
}

func (fake *FakeCluster) GetRegionCalls(stub func() string) {
	fake.getRegionMutex.Lock()
	defer fake.getRegionMutex.Unlock()
	fake.GetRegionStub = stub
}

func (fake *FakeCluster) GetRegionReturns(result1 string) {
	fake.getRegionMutex.Lock()
	defer fake.getRegionMutex.Unlock()
	fake.GetRegionStub = nil
	fake.getRegionReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeCluster) GetRegionReturnsOnCall(i int, result1 string) {
	fake.getRegionMutex.Lock()
	defer fake.getRegionMutex.Unlock()
	fake.GetRegionStub = nil
	if fake.getRegionReturnsOnCall == nil {
		fake.getRegionReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.getRegionReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeCluster) GetRole() string {
	fake.getRoleMutex.Lock()
	ret, specificReturn := fake.getRoleReturnsOnCall[len(fake.getRoleArgsForCall)]
	fake.getRoleArgsForCall = append(fake.getRoleArgsForCall, struct {
	}{})
	stub := fake.GetRoleStub
	fakeReturns := fake.getRoleReturns
	fake.recordInvocation("GetRole", []interface{}{})
	fake.getRoleMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeCluster) GetRoleCallCount() int {
	fake.getRoleMutex.RLock()
	defer fake.getRoleMutex.RUnlock()
	return len(fake.getRoleArgsForCall)
}

func (fake *FakeCluster) GetRoleCalls(stub func() string) {
	fake.getRoleMutex.Lock()
	defer fake.getRoleMutex.Unlock()
	fake.GetRoleStub = stub
}

func (fake *FakeCluster) GetRoleReturns(result1 string) {
	fake.getRoleMutex.Lock()
	defer fake.getRoleMutex.Unlock()
	fake.GetRoleStub = nil
	fake.getRoleReturns = struct {
		result1 string
	}{result1}
}

func (fake *FakeCluster) GetRoleReturnsOnCall(i int, result1 string) {
	fake.getRoleMutex.Lock()
	defer fake.getRoleMutex.Unlock()
	fake.GetRoleStub = nil
	if fake.getRoleReturnsOnCall == nil {
		fake.getRoleReturnsOnCall = make(map[int]struct {
			result1 string
		})
	}
	fake.getRoleReturnsOnCall[i] = struct {
		result1 string
	}{result1}
}

func (fake *FakeCluster) GetTags() map[string]string {
	fake.getTagsMutex.Lock()
	ret, specificReturn := fake.getTagsReturnsOnCall[len(fake.getTagsArgsForCall)]
	fake.getTagsArgsForCall = append(fake.getTagsArgsForCall, struct {
	}{})
	stub := fake.GetTagsStub
	fakeReturns := fake.getTagsReturns
	fake.recordInvocation("GetTags", []interface{}{})
	fake.getTagsMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeCluster) GetTagsCallCount() int {
	fake.getTagsMutex.RLock()
	defer fake.getTagsMutex.RUnlock()
	return len(fake.getTagsArgsForCall)
}

func (fake *FakeCluster) GetTagsCalls(stub func() map[string]string) {
	fake.getTagsMutex.Lock()
	defer fake.getTagsMutex.Unlock()
	fake.GetTagsStub = stub
}

func (fake *FakeCluster) GetTagsReturns(result1 map[string]string) {
	fake.getTagsMutex.Lock()
	defer fake.getTagsMutex.Unlock()
	fake.GetTagsStub = nil
	fake.getTagsReturns = struct {
		result1 map[string]string
	}{result1}
}

func (fake *FakeCluster) GetTagsReturnsOnCall(i int, result1 map[string]string) {
	fake.getTagsMutex.Lock()
	defer fake.getTagsMutex.Unlock()
	fake.GetTagsStub = nil
	if fake.getTagsReturnsOnCall == nil {
		fake.getTagsReturnsOnCall = make(map[int]struct {
			result1 map[string]string
		})
	}
	fake.getTagsReturnsOnCall[i] = struct {
		result1 map[string]string
	}{result1}
}

func (fake *FakeCluster) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getBaseDomainMutex.RLock()
	defer fake.getBaseDomainMutex.RUnlock()
	fake.getNameMutex.RLock()
	defer fake.getNameMutex.RUnlock()
	fake.getNamespaceMutex.RLock()
	defer fake.getNamespaceMutex.RUnlock()
	fake.getRegionMutex.RLock()
	defer fake.getRegionMutex.RUnlock()
	fake.getRoleMutex.RLock()
	defer fake.getRoleMutex.RUnlock()
	fake.getTagsMutex.RLock()
	defer fake.getTagsMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeCluster) recordInvocation(key string, args []interface{}) {
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

var _ cluster.Cluster = new(FakeCluster)
