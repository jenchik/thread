package thread

import (
	"sync/atomic"
)

type (
	LockedIface interface {
		IsLocked() bool
	}

	AcquireReleaseIface interface {
		Acquire()
		Release()
	}

	AcquireReleaseLockIface interface {
		Acquire() bool
		Release() bool
	}

	SemaphoreInt int32
)

var (
	_ AcquireReleaseLockIface = ((*SemaphoreInt)(nil))
	_ LockedIface             = ((*SemaphoreInt)(nil))
)

func (s *SemaphoreInt) IsLocked() bool {
	return atomic.LoadInt32((*int32)(s)) == 1
}

func (s *SemaphoreInt) Acquire() bool {
	return atomic.CompareAndSwapInt32((*int32)(s), 0, 1)
}

func (s *SemaphoreInt) Release() bool {
	return atomic.CompareAndSwapInt32((*int32)(s), 1, 0)
}
