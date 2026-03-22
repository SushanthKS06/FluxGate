package loadbalancer

import (
	"sync/atomic"
)

type RoundRobin struct {
	upstreams []string
	index     int64
}

func NewRoundRobin(upstreams []string) *RoundRobin {
	return &RoundRobin{
		upstreams: upstreams,
		index:     -1,
	}
}

func (rr *RoundRobin) Next() string {
	n := atomic.AddInt64(&rr.index, 1)
	return rr.upstreams[n%int64(len(rr.upstreams))]
}
