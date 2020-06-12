// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import git "github.com/capnfabs/grouse/internal/git"
import mock "github.com/stretchr/testify/mock"

// WorktreeRepository is an autogenerated mock type for the WorktreeRepository type
type WorktreeRepository struct {
	mock.Mock
}

// Checkout provides a mock function with given fields: commit
func (_m *WorktreeRepository) Checkout(commit git.ResolvedCommit) error {
	ret := _m.Called(commit)

	var r0 error
	if rf, ok := ret.Get(0).(func(git.ResolvedCommit) error); ok {
		r0 = rf(commit)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RecursiveSharedCloneTo provides a mock function with given fields: dst
func (_m *WorktreeRepository) RecursiveSharedCloneTo(dst string) (git.WorktreeRepository, error) {
	ret := _m.Called(dst)

	var r0 git.WorktreeRepository
	if rf, ok := ret.Get(0).(func(string) git.WorktreeRepository); ok {
		r0 = rf(dst)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(git.WorktreeRepository)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(dst)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Remove provides a mock function with given fields:
func (_m *WorktreeRepository) Remove() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ResolveCommit provides a mock function with given fields: ref
func (_m *WorktreeRepository) ResolveCommit(ref string) (git.ResolvedUserRef, error) {
	ret := _m.Called(ref)

	var r0 git.ResolvedUserRef
	if rf, ok := ret.Get(0).(func(string) git.ResolvedUserRef); ok {
		r0 = rf(ref)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(git.ResolvedUserRef)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(ref)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RootDir provides a mock function with given fields:
func (_m *WorktreeRepository) RootDir() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}