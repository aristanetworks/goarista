package hash

import (
	"encoding/binary"
	"fmt"
	"hash/maphash"
	"strings"
	"sync"
	"testing"
)

func (m *Map[K, E]) DebugString() string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "count: %d, buckets: %d, overflows: %d\n", m.count, len(m.buckets), m.noverflow)

	for i, b := range m.buckets {
		fmt.Fprintf(&buf, "bucket: %d\n", i)
		b := &b
		for {
			for i := uintptr(0); i < bucketCnt; i++ {
				seen := map[uint8]struct{}{}
				switch b.tophash[i] {
				case emptyRest:
					buf.WriteString("  emptyRest\n")
				case emptyOne:
					buf.WriteString("  emptyOne\n")
				case evacuatedX:
					buf.WriteString("  evacuatedX?\n")
				case evacuatedY:
					buf.WriteString("  evacuatedY?\n")
				case evacuatedEmpty:
					buf.WriteString("  evacuatedEmpty?\n")
				default:
					var s string
					if _, ok := seen[b.tophash[i]]; ok {
						s = " duplicate"
					} else {
						seen[b.tophash[i]] = struct{}{}
					}
					fmt.Fprintf(&buf, "  0x%02x"+s+"\n", b.tophash[i])
				}
			}
			if b.overflow == nil {
				break
			}
			buf.WriteString("overflow->\n")
			b = b.overflow
		}
	}

	return buf.String()
}

func newIntHasher() func(a int) uint64 {
	seed := maphash.MakeSeed()
	return func(a int) uint64 {
		var (
			buf [8]byte
			h   maphash.Hash
		)
		h.SetSeed(seed)
		binary.LittleEndian.PutUint64(buf[:], uint64(a))
		h.Write(buf[:])
		return h.Sum64()
	}
}

func TestSetGetDelete(t *testing.T) {
	const count = 1000000
	t.Run("nohint", func(t *testing.T) {
		m := New[int, int](func(a int, b int) bool { return a == b }, newIntHasher())
		t.Logf("Buckets: %d Unused-overflow: %d", len(m.buckets), cap(m.buckets)-len(m.buckets))
		for i := 0; i < count; i++ {
			m.Set(i, i)
			if v, ok := m.Get(i); !ok {
				t.Errorf("got not ok for %d", i)
			} else if v != i {
				t.Errorf("unexpected value for %d: %d", i, v)
			}
			if m.Len() != i+1 {
				t.Errorf("expected len: %d got: %d", i+1, m.Len())
			}
		}
		t.Logf("Buckets: %d Unused-overflow: %d", len(m.buckets), cap(m.buckets)-len(m.buckets))
		t.Log("Overflow:", m.noverflow)
		for i := 0; i < count; i++ {
			if v, ok := m.Get(i); !ok {
				t.Errorf("got not ok for %d", i)
			} else if v != i {
				t.Errorf("unexpected value for %d: %d", i, v)
			}
			if m.Len() != count {
				t.Errorf("expected len: %d got: %d", count, m.Len())
			}

		}
		for i := 0; i < count; i++ {
			if v, ok := m.Get(i); !ok {
				t.Errorf("got not ok for %d", i)
			} else if v != i {
				t.Errorf("unexpected value for %d: %d", i, v)
			}

			m.Delete(i)

			if v, ok := m.Get(i); ok {
				t.Errorf("found %d: %d, but it should have been deleted", i, v)
			}
			if m.Len() != count-i-1 {
				t.Errorf("expected len: %d got: %d", count, m.Len())
			}
		}
	})
	t.Run("hint", func(t *testing.T) {
		m := NewSizeHint[int, int](count, func(a int, b int) bool { return a == b }, newIntHasher())
		t.Logf("Buckets: %d Unused-overflow: %d", len(m.buckets), cap(m.buckets)-len(m.buckets))
		for i := 0; i < count; i++ {
			m.Set(i, i)
			if v, ok := m.Get(i); !ok {
				t.Errorf("got not ok for %d", i)
			} else if v != i {
				t.Errorf("unexpected value for %d: %d", i, v)
			}
			if m.Len() != i+1 {
				t.Errorf("expected len: %d got: %d", i+1, m.Len())
			}
		}
		t.Logf("Buckets: %d Unused-overflow: %d", len(m.buckets), cap(m.buckets)-len(m.buckets))
		t.Log("Overflow:", m.noverflow)
		for i := 0; i < count; i++ {
			if v, ok := m.Get(i); !ok {
				t.Errorf("got not ok for %d", i)
			} else if v != i {
				t.Errorf("unexpected value for %d: %d", i, v)
			}
			if m.Len() != count {
				t.Errorf("expected len: %d got: %d", count, m.Len())
			}

		}
		for i := 0; i < count; i++ {
			if v, ok := m.Get(i); !ok {
				t.Errorf("got not ok for %d", i)
			} else if v != i {
				t.Errorf("unexpected value for %d: %d", i, v)
			}

			m.Delete(i)

			if v, ok := m.Get(i); ok {
				t.Errorf("found %d: %d, but it should have been deleted", i, v)
			}
			if m.Len() != count-i-1 {
				t.Errorf("expected len: %d got: %d", count, m.Len())
			}
		}
	})
}

func BenchmarkGrow(b *testing.B) {
	b.Run("hint", func(b *testing.B) {
		b.ReportAllocs()
		m := NewSizeHint[int, int](b.N, func(a int, b int) bool { return a == b }, newIntHasher())
		for i := 0; i < b.N; i++ {
			m.Set(i, i)
		}
	})
	b.Run("nohint", func(b *testing.B) {
		b.ReportAllocs()
		m := New[int, int](func(a int, b int) bool { return a == b }, newIntHasher())
		for i := 0; i < b.N; i++ {
			m.Set(i, i)
		}
	})

	b.Run("std:hint", func(b *testing.B) {
		b.ReportAllocs()
		m := make(map[int]int, b.N)
		for i := 0; i < b.N; i++ {
			m[i] = i
		}
	})
	b.Run("std:nohint", func(b *testing.B) {
		b.ReportAllocs()
		m := map[int]int{}
		for i := 0; i < b.N; i++ {
			m[i] = i
		}
	})
}

func TestGetIterateRace(t *testing.T) {
	m := NewSizeHint[int, int](100, func(a int, b int) bool { return a == b }, newIntHasher())
	for i := 0; i < 100; i++ {
		m.Set(i, i)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < 100; i++ {
			v, ok := m.Get(i)
			if !ok || v != i {
				t.Errorf("expected: %d got: %d, %t", i, v, ok)
			}
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		for i := 0; i < 100; i++ {
			v, ok := m.Get(i)
			if !ok || v != i {
				t.Errorf("expected: %d got: %d, %t", i, v, ok)
			}
		}
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		for i := 0; i < 100; i++ {
			iter := m.Iter()
			if !iter.Next() {
				t.Error("unexpected end of iter")
			}
		}
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		for i := 0; i < 100; i++ {
			iter := m.Iter()
			if !iter.Next() {
				t.Error("unexpected end of iter")
			}
		}
		wg.Done()
	}()
	wg.Wait()
}
