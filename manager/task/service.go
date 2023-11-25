package task

import (
	"context"
	"sync"
	"time"
)

type service struct {
	lock     *sync.RWMutex
	statuses map[string]Status
	cleanup  map[string]func()
}

type TaskService interface {
	Add(id string)
	Failed(id string, details interface{})
	Succeeded(id string, details interface{})
	Get(id string) Status
}

func CreateTaskService() TaskService {
	return &service{
		lock:     &sync.RWMutex{},
		statuses: map[string]Status{},
		cleanup:  map[string]func(){},
	}
}

func (s *service) Add(id string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.statuses[id] = Pending{StartedAt_: time.Now().UnixMicro()}
}

func (s *service) Failed(id string, details interface{}) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.statuses[id] = Failed{
		ResultStatus{
			StartedAt_: s.statuses[id].StartedAt(),
			FinishedAt: time.Now().UnixMicro(),
			Details:    details,
		},
	}

	s.poke(id)
}

func (s *service) Succeeded(id string, details interface{}) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.statuses[id] = Succeeded{
		ResultStatus: ResultStatus{
			StartedAt_: s.statuses[id].StartedAt(),
			FinishedAt: time.Now().UnixMicro(),
			Details:    details,
		},
	}

	s.poke(id)
}

func (s *service) Get(id string) Status {
	s.lock.Lock()
	defer s.lock.Unlock()

	status, ok := s.statuses[id]

	if !ok {
		return nil
	}

	s.poke(id)
	return status
}

func (s *service) poke(id string) {
	cancel := s.cleanup[id]
	if cancel != nil {
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-time.After(15 * time.Minute):
			s.lock.Lock()
			defer s.lock.Unlock()

			delete(s.cleanup, id)
			delete(s.statuses, id)
		case <-ctx.Done():
			return
		}
	}()

	s.cleanup[id] = cancel
}
