// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"
import wire "gitlab.dusk.network/dusk-core/dusk-go/pkg/p2p/wire"

// Committee is an autogenerated mock type for the Committee type
type Committee struct {
	mock.Mock
}

// IsMember provides a mock function with given fields: _a0, _a1, _a2
func (_m *Committee) IsMember(_a0 []byte, _a1 uint64, _a2 uint8) bool {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 bool
	if rf, ok := ret.Get(0).(func([]byte, uint64, uint8) bool); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Priority provides a mock function with given fields: _a0, _a1
func (_m *Committee) Priority(_a0 wire.Event, _a1 wire.Event) bool {
	ret := _m.Called(_a0, _a1)

	var r0 bool
	if rf, ok := ret.Get(0).(func(wire.Event, wire.Event) bool); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Quorum provides a mock function with given fields:
func (_m *Committee) Quorum() int {
	ret := _m.Called()

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// ReportAbsentees provides a mock function with given fields: _a0, _a1, _a2
func (_m *Committee) ReportAbsentees(_a0 []wire.Event, _a1 uint64, _a2 uint8) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func([]wire.Event, uint64, uint8) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
