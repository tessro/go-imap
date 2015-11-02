package imap

import (
	"fmt"
	"strconv"
	"strings"
)

// Sentinel for IMAP '*'
const Star = -1

type SequenceRange [2]int

type SequenceSet struct {
	ranges []SequenceRange
}

func (r *SequenceRange) String() string {
	if r[0] == r[1] {
		return strconv.Itoa(r[0])
	} else {
		return fmt.Sprintf("%d:%d", r.Min(), r.Max())
	}
}

func (s *SequenceSet) String() string {
	strs := make([]string, len(s.ranges))
	for i, r := range s.ranges {
		strs[i] = r.String()
	}
	return strings.Join(strs, ",")
}

func (r *SequenceRange) Min() int {
	if r[0] == Star {
		return r[1]
	} else if r[1] == Star {
		return r[0]
	}

	if r[0] <= r[1] {
		return r[0]
	} else {
		return r[1]
	}
}

func (r *SequenceRange) Max() int {
	if r[1] == Star || r[0] == Star {
		return Star
	}

	if r[1] >= r[0] {
		return r[1]
	} else {
		return r[0]
	}
}

func (r *SequenceRange) Len() int {
	return r.Max() - r.Min() + 1
}

func NewSequenceSetWithRange(rng SequenceRange) *SequenceSet {
	return &SequenceSet{
		ranges: []SequenceRange{rng},
	}
}

func (s *SequenceSet) Len() int {
	l := 0
	for _, rng := range s.ranges {
		l += rng.Len()
	}
	return l
}

func (s *SequenceSet) Max() int {
	max := 0
	for _, rng := range s.ranges {
		if rng.Max() > max {
			max = rng.Max()
		}
	}
	return max
}

func (s *SequenceSet) Append(rng SequenceRange) {
	s.ranges = append(s.ranges, rng)
}

func (s *SequenceSet) Foreach(max int, callback func(int) error) error {
	for _, rng := range s.ranges {
		localMin := rng.Min()
		localMax := rng.Max()
		if localMin == Star {
			localMin = max
		}
		if localMax == Star || localMax > max {
			localMax = max
		}

		for i := localMin; i <= localMax; i++ {
			err := callback(i)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
