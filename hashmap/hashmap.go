// Copyright (c) 2020 Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package hashmap

import "math/bits"

// Hashable represents the key for an entry in a Map that cannot natively be hashed
type Hashable interface {
	Hash() uint64
	Equal(other interface{}) bool
}

// Hashmap implements a hashmap
type Hashmap[K any, V any] struct {
	seed    uint64
	entries []entry[K, V]
	length  int
	hash    func(K) uint64
	equal   func(K, K) bool
}

func New[K any, V any](size uint, hash func(K) uint64, equal func(K, K) bool) *Hashmap[K, V] {
	var entries []entry[K, V]
	if size != 0 {
		entries = make([]entry[K, V], 1<<bits.Len(size-1))
	}
	return &Hashmap[K, V]{entries: entries, hash: hash, equal: equal}
}

type entry[K any, V any] struct {
	hash      uint64
	key       K
	value     V
	occupied  bool
	tombstone bool
}

// Len returns the length of m.
func (m *Hashmap[K, V]) Len() int {
	return m.length
}

func (m *Hashmap[K, V]) mask() int {
	return len(m.entries) - 1
}

func (m *Hashmap[K, V]) position(hash uint64) int {
	return int((hash ^ m.seed)) & m.mask()
}

// Set associates k with v in m.
func (m *Hashmap[K, V]) Set(k K, v V) {
	capacity := len(m.entries)
	if capacity == 0 {
		m.resize(4)
	} else if m.length >= int(float64(capacity)*0.9) {
		m.resize(capacity * 2)
	}
	m.set(m.hash(k), k, v)
}

func (m *Hashmap[K, V]) set(hash uint64, k K, v V) {
	position := m.position(hash)
	var distance int
	for {
		existing := &m.entries[position]
		if !existing.occupied {
			m.entries[position] = entry[K, V]{hash: hash, key: k, value: v, occupied: true}
			m.length++
			return
		} else if existing.hash == hash && m.equal(existing.key, k) {
			existing.value = v
			return
		}

		existingDistance := position - m.position(existing.hash)
		if existingDistance < 0 {
			existingDistance += len(m.entries)
		}
		if distance > existingDistance {
			// k is further from its desired position than existing.k,
			// steal it's spot and find a new place for existing.
			if existing.tombstone {
				m.entries[position] = entry[K, V]{hash: hash, key: k, value: v, occupied: true}
				m.length++
				return
			}
			hash, existing.hash = existing.hash, hash
			k, existing.key = existing.key, k
			v, existing.value = existing.value, v
			distance = existingDistance
		} else if distance == existingDistance && existing.tombstone {
			m.entries[position] = entry[K, V]{hash: hash, key: k, value: v, occupied: true}
			m.length++
			return
		}

		distance++
		position = (position + 1) & m.mask()
	}
}

// Get gets the value associated with k
func (m *Hashmap[K, V]) Get(k K) (V, bool) {
	ent := m.getRef(k)
	if ent == nil {
		var v V
		return v, false
	}
	return ent.value, true
}

func (m *Hashmap[K, V]) getRef(k K) *entry[K, V] {
	hash := m.hash(k)
	position := m.position(hash)
	var distance int
	for {
		ent := &m.entries[position]
		if !ent.occupied {
			return nil
		}
		entDistance := position - m.position(ent.hash)
		if entDistance < 0 {
			entDistance += len(m.entries)
		}
		if distance > entDistance {
			// Our distance has exceeded this entry's distance, we
			// would have found our key by now if it was present.
			return nil
		}
		if ent.hash == hash && m.equal(ent.key, k) {
			return ent
		}
		distance++
		position = (position + 1) & m.mask()
	}
}

// Delete removes k from m
func (m *Hashmap[K, V]) Delete(k K) {
	ent := m.getRef(k)
	if ent == nil {
		return
	}
	// Set the entry to a tombstone. We keep the entry's hash set, so
	// that this entry's distance can still be calculated.
	var (
		nilK K
		nilV V
	)
	ent.key = nilK
	ent.value = nilV
	ent.tombstone = true
	m.length--
}

func (m *Hashmap[K, V]) resize(size int) {
	oldEntries := m.entries
	m.entries = make([]entry[K, V], size)
	m.length = 0
	for _, ent := range oldEntries {
		if !ent.occupied || ent.tombstone {
			continue
		}
		m.set(ent.hash, ent.key, ent.value)
	}
}
