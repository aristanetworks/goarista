// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hash

// This file contains the implementation of Go's map type.
//
// A map is just a hash table. The data is arranged
// into an array of buckets. Each bucket contains up to
// 8 key/elem pairs. The low-order bits of the hash are
// used to select a bucket. Each bucket contains a few
// high-order bits of each hash to distinguish the entries
// within a single bucket.
//
// If more than 8 keys hash to a bucket, we chain on
// extra buckets.
//
// When the hashtable grows, we allocate a new array
// of buckets twice as big. Buckets are incrementally
// copied from the old bucket array to the new bucket array.
//
// Map iterators walk through the array of buckets and
// return the keys in walk order (bucket #, then overflow
// chain order, then bucket index).  To maintain iteration
// semantics, we never move keys within their bucket (if
// we did, keys might be returned 0 or 2 times).  When
// growing the table, iterators remain iterating through the
// old table and must check the new table if the bucket
// they are iterating through has been moved ("evacuated")
// to the new table.

// Picking loadFactor: too large and we have lots of overflow
// buckets, too small and we waste a lot of space. I wrote
// a simple program to check some stats for different loads:
// (64-bit, 8 byte keys and elems)
//  loadFactor    %overflow  bytes/entry     hitprobe    missprobe
//        4.00         2.13        20.77         3.00         4.00
//        4.50         4.05        17.30         3.25         4.50
//        5.00         6.85        14.77         3.50         5.00
//        5.50        10.55        12.94         3.75         5.50
//        6.00        15.27        11.67         4.00         6.00
//        6.50        20.90        10.79         4.25         6.50
//        7.00        27.14        10.15         4.50         7.00
//        7.50        34.03         9.73         4.75         7.50
//        8.00        41.10         9.40         5.00         8.00
//
// %overflow   = percentage of buckets which have an overflow bucket
// bytes/entry = overhead bytes used per key/elem pair
// hitprobe    = # of entries to check when looking up a present key
// missprobe   = # of entries to check when looking up an absent key
//
// Keep in mind this data is for maximally loaded tables, i.e. just
// before the table grows. Typical tables will be somewhat less loaded.

import (
	"sync/atomic"

	"golang.org/x/exp/rand"
)

const (
	// Maximum number of key/elem pairs a bucket can hold.
	bucketCntBits = 3
	bucketCnt     = 1 << bucketCntBits

	// Maximum average load of a bucket that triggers growth is 6.5.
	// Represent as loadFactorNum/loadFactorDen, to allow integer math.
	loadFactorNum = 13
	loadFactorDen = 2

	// Possible tophash values. We reserve a few possibilities for special marks.
	// Each bucket (including its overflow buckets, if any) will have either all or none of its
	// entries in the evacuated* states (except during the evacuate() method, which only happens
	// during map writes and thus no one else can observe the map during that time).
	emptyRest      = 0 // this cell is empty, and there are no more non-empty cells at higher indexes or overflows.
	emptyOne       = 1 // this cell is empty
	evacuatedX     = 2 // key/elem is valid.  Entry has been evacuated to first half of larger table.
	evacuatedY     = 3 // same as above, but evacuated to second half of larger table.
	evacuatedEmpty = 4 // cell is empty, bucket is evacuated.
	minTopHash     = 5 // minimum tophash for a normal filled cell.

	// flags
	iterator     = 1 // there may be an iterator using buckets
	oldIterator  = 2 // there may be an iterator using oldbuckets
	hashWriting  = 4 // a goroutine is writing to the map
	sameSizeGrow = 8 // the current map growth is to a new map of the same size

	// sentinel bucket ID for iterator checks
	noCheck = -1
)

// isEmpty reports whether the given tophash array entry represents an empty bucket entry.
func isEmpty(x uint8) bool {
	return x <= emptyOne
}

type Map[K, E any] struct {
	count     int    // # live cells == size of map
	flags     uint32 // Only the first 8 bits are used. uint32 is used here to allow use of atomic.*Uint32 operations
	noverflow uint32 // number of overflow buckets; see incrnoverflow for details

	buckets    []bucket[K, E]  // array of 2^B Buckets. may be nil if count==0.
	oldbuckets *[]bucket[K, E] // previous bucket array of half the size, non-nil only when growing
	nevacuate  uint64          // progress counter for evacuation (buckets less than this have been evacuated)

	hasher func(k K) uint64
	equals func(a, b K) bool
}

type bucket[K, E any] struct {
	// tophash generally contains the top byte of the hash value
	// for each key in this bucket. If tophash[0] < minTopHash,
	// tophash[0] is a bucket evacuation state instead.
	tophash [bucketCnt]uint8
	// Followed by bucketCnt keys and then bucketCnt elems.
	// NOTE: packing all the keys together and then all the elems together makes the
	// code a bit more complicated than alternating key/elem/key/elem/... but it allows
	// us to eliminate padding which would be needed for, e.g., map[int64]int8.
	keys  [bucketCnt]K
	elems [bucketCnt]E
	// Followed by an overflow pointer.
	overflow *bucket[K, E]
}

type Iterator[K, E any] struct {
	key         K
	elem        E
	m           *Map[K, E]
	buckets     []bucket[K, E]
	bptr        *bucket[K, E]
	startBucket int
	offset      uint8
	wrapped     bool
	i           uint8
	bucket      int
	checkBucket int
}

func (i *Iterator[K, E]) Key() K {
	return i.key
}

func (i *Iterator[K, E]) Elem() E {
	return i.elem
}

// bucketShift returns 1<<b, optimized for code generation.
func bucketShift(b uint8) uint64 {
	// Masking the shift amount allows overflow checks to be elided.
	return uint64(1) << (b & 63)
}

// tophash calculates the tophash value for hash.
func tophash(hash uint64) uint8 {
	top := uint8(hash >> 56)
	if top < minTopHash {
		top += minTopHash
	}
	return top
}

func evacuated[K, E any](b *bucket[K, E]) bool {
	h := b.tophash[0]
	return h > emptyOne && h < minTopHash
}

func (m *Map[K, E]) newoverflow(b *bucket[K, E]) *bucket[K, E] {
	var ovf *bucket[K, E]
	if len(m.buckets) < cap(m.buckets) {
		ovf = &m.buckets[:cap(m.buckets)][cap(m.buckets)-1]
		m.buckets = m.buckets[: len(m.buckets) : cap(m.buckets)-1]
	} else {
		ovf = &bucket[K, E]{}
	}
	b.overflow = ovf
	m.noverflow++
	return b.overflow
}

func New[K, E any](equals func(a, b K) bool, hasher func(K) uint64) *Map[K, E] {
	return &Map[K, E]{hasher: hasher, equals: equals}
}

func NewSizeHint[K, E any](hint int, equals func(a, b K) bool, hasher func(K) uint64) *Map[K, E] {
	if hint <= 0 {
		return New[K, E](equals, hasher)
	}
	nbuckets := 1
	for overLoadFactor(hint, nbuckets) {
		nbuckets *= 2
	}
	buckets := makeBucketArray[K, E](nbuckets)

	return &Map[K, E]{buckets: buckets, hasher: hasher, equals: equals}
}

func makeBucketArray[K, E any](nbuckets int) []bucket[K, E] {
	if nbuckets&(nbuckets-1) != 0 {
		panic("nbuckets is not power of 2")
	}
	var newbuckets []bucket[K, E]
	// Preallocate expected overflow buckets at the end of the buckets
	// slice
	additional := nbuckets >> 4
	if additional == 0 {
		newbuckets = make([]bucket[K, E], nbuckets)
	} else {
		// Using append here allows the go runtime to round up the
		// capacity of newbuckets to fit the next size class, giving
		// us some free buckets we don't need to allocate later.
		newbuckets = append([]bucket[K, E](nil),
			make([]bucket[K, E], nbuckets+additional)...)
		newbuckets = newbuckets[:nbuckets]
	}
	return newbuckets
}

func (m *Map[K, V]) Len() int {
	if m == nil {
		return 0
	}
	return m.count
}

func (m *Map[K, E]) Get(key K) (E, bool) {
	var zeroE E
	if m == nil || m.count == 0 {
		return zeroE, false
	}
	// if m.flags&hashWriting != 0 {
	// 	panic("concurrent map read and map write")
	// }
	hash := m.hasher(key)
	mask := m.bucketMask()
	b := &m.buckets[int(hash&mask)]
	if c := m.oldbuckets; c != nil {
		if !m.sameSizeGrow() {
			// There used to be half as many buckets; mask down one more power of two.
			mask >>= 1
		}
		oldb := &(*c)[int(hash&mask)]
		if !evacuated(oldb) {
			b = oldb
		}
	}
	top := tophash(hash)
bucketloop:
	for ; b != nil; b = b.overflow {
		for i := uintptr(0); i < bucketCnt; i++ {
			if b.tophash[i] != top {
				if b.tophash[i] == emptyRest {
					break bucketloop
				}
				continue
			}
			if m.equals(key, b.keys[i]) {
				return b.elems[i], true
			}
		}
	}
	return zeroE, false
}

// returns both key and elem. Used by map iterator
func (m *Map[K, E]) mapaccessK(key K) (*K, *E) {
	if m == nil || m.count == 0 {
		return nil, nil
	}
	if m.flags&hashWriting != 0 {
		panic("concurrent map read and map write")
	}
	hash := m.hasher(key)
	mask := m.bucketMask()
	b := &m.buckets[int(hash&mask)]
	if c := m.oldbuckets; c != nil {
		if !m.sameSizeGrow() {
			// There used to be half as many buckets; mask down one more power of two.
			mask >>= 1
		}
		oldb := &(*c)[int(hash&mask)]
		if !evacuated(oldb) {
			b = oldb
		}
	}
	top := tophash(hash)
bucketloop:
	for ; b != nil; b = b.overflow {
		for i := uintptr(0); i < bucketCnt; i++ {
			if b.tophash[i] != top {
				if b.tophash[i] == emptyRest {
					break bucketloop
				}
				continue
			}
			if m.equals(key, b.keys[i]) {
				return &b.keys[i], &b.elems[i]
			}
		}
	}
	return nil, nil
}

func (m *Map[K, E]) Set(key K, elem E) {
	if m == nil {
		// We have to panic here rather than initialize an empty map
		// because we need the user to pass in hash and equals
		// functions
		panic("Set called on nil map")
	}
	if m.flags&hashWriting != 0 {
		panic("concurrent map writes")
	}
	hash := m.hasher(key)
	// Set hashWriting after calling t.hasher, since t.hasher may panic,
	// in which case we have not actually done a write.
	m.flags ^= hashWriting

	if m.buckets == nil {
		m.buckets = makeBucketArray[K, E](1)
	}

again:
	mask := m.bucketMask()
	bucket := hash & mask
	if m.growing() {
		m.growWork(bucket)
	}

	b := &m.buckets[hash&mask]
	top := tophash(hash)

	var inserti *uint8
	var insertk *K
	var inserte *E
bucketloop:
	for {
		for i := uintptr(0); i < bucketCnt; i++ {
			if b.tophash[i] != top {
				if isEmpty(b.tophash[i]) && inserti == nil {
					inserti = &b.tophash[i]
					insertk = &b.keys[i]
					inserte = &b.elems[i]
				}
				if b.tophash[i] == emptyRest {
					break bucketloop
				}
				continue
			}
			k := b.keys[i]
			if !m.equals(key, k) {
				continue
			}
			// already have a mapping for key. Update it.
			b.keys[i] = key
			b.elems[i] = elem
			goto done
		}
		ovf := b.overflow
		if ovf == nil {
			break
		}
		b = ovf
	}

	// Did not find mapping for key. Allocate new cell & add entry.

	// If we hit the max load factor or we have too many overflow buckets,
	// and we're not already in the middle of growing, start growing.
	if !m.growing() && (overLoadFactor(m.count+1, len(m.buckets)) ||
		tooManyOverflowBuckets(m.noverflow, len(m.buckets))) {
		m.hashGrow()
		goto again // Growing the table invalidates everything, so try again
	}

	if inserti == nil {
		// The current bucket and all the overflow buckets connected to it are full, allocate a new one.
		newb := m.newoverflow(b)
		inserti = &newb.tophash[0]
		insertk = &newb.keys[0]
		inserte = &newb.elems[0]
	}

	// store new key/elem at insert position
	*insertk = key
	*inserte = elem
	*inserti = top
	m.count++

done:
	if m.flags&hashWriting == 0 {
		panic("concurrent map writes")
	}
	m.flags &^= hashWriting
}

func (m *Map[K, E]) Delete(key K) {
	if m == nil || m.count == 0 {
		return
	}
	hash := m.hasher(key)

	// Set hashWriting after calling t.hasher, since t.hasher may panic,
	// in which case we have not actually done a write (delete).
	m.flags ^= hashWriting
	bucket := hash & m.bucketMask()
	if m.growing() {
		m.growWork(bucket)
	}
	b := &m.buckets[bucket]
	bOrig := b
	top := tophash(hash)
search:
	for ; b != nil; b = b.overflow {
		for i := uintptr(0); i < bucketCnt; i++ {
			if b.tophash[i] != top {
				if b.tophash[i] == emptyRest {
					break search
				}
				continue
			}
			k := b.keys[i]
			if !m.equals(key, k) {
				continue
			}
			var (
				zeroK K
				zeroE E
			)
			// Clear key and elem in case they have pointers
			b.keys[i] = zeroK
			b.elems[i] = zeroE
			b.tophash[i] = emptyOne
			// If the bucket now ends in a bunch of emptyOne states,
			// change those to emptyRest states.
			// It would be nice to make this a separate function, but
			// for loops are not currently inlineable.
			if i == bucketCnt-1 {
				if b.overflow != nil && b.overflow.tophash[0] != emptyRest {
					goto notLast
				}
			} else {
				if b.tophash[i+1] != emptyRest {
					goto notLast
				}
			}
			for {
				b.tophash[i] = emptyRest
				if i == 0 {
					if b == bOrig {
						break // beginning of initial bucket, we're done.
					}
					// Find previous bucket, continue at its last entry.
					c := b
					for b = bOrig; b.overflow != c; b = b.overflow {
					}
					i = bucketCnt - 1
				} else {
					i--
				}
				if b.tophash[i] != emptyOne {
					break
				}
			}
		notLast:
			m.count--
			break search
		}

	}

	if m.flags&hashWriting == 0 {
		panic("concurrent map writes")
	}
	m.flags &^= hashWriting
}

func (m *Map[K, E]) Iter() *Iterator[K, E] {
	if m == nil || m.count == 0 {
		return nil
	}
	r := rand.Uint64()
	it := Iterator[K, E]{
		m:           m,
		buckets:     m.buckets,
		startBucket: int(r & m.bucketMask()),
		offset:      uint8(r >> (64 - bucketCntBits)),
	}
	atomicOr(&m.flags, iterator|oldIterator)
	return &it
}

func atomicOr(flags *uint32, or uint32) {
	old := atomic.LoadUint32(flags)
	for !atomic.CompareAndSwapUint32(flags, old, old|or) {
		// force re-reading from memory
		old = atomic.LoadUint32(flags)
	}
}

func (it *Iterator[K, E]) Next() bool {
	m := it.m
	// if m.flags&hashWriting != 0 {
	// 	panic("concurrent map iteration and map write")
	// }
	bucket := it.bucket
	b := it.bptr
	i := it.i
	checkBucket := it.checkBucket

next:
	if b == nil {
		if bucket == it.startBucket && it.wrapped {
			// end of iteration
			var (
				zeroK K
				zeroE E
			)
			it.key = zeroK
			it.elem = zeroE
			return false
		}
		if m.growing() && len(it.buckets) == len(m.buckets) {
			// Iterator was started in the middle of a grow, and the grow isn't done yet.
			// If the bucket we're looking at hasn't been filled in yet (i.e. the old
			// bucket hasn't been evacuated) then we need to iterate through the old
			// bucket and only return the ones that will be migrated to this bucket.
			oldbucket := uint64(bucket) & it.m.oldbucketmask()
			b = &(*m.oldbuckets)[oldbucket]
			if !evacuated(b) {
				checkBucket = bucket
			} else {
				b = &it.buckets[bucket]
				checkBucket = noCheck
			}
		} else {
			b = &it.buckets[bucket]
			checkBucket = noCheck
		}
		bucket++
		if bucket == len(it.buckets) {
			bucket = 0
			it.wrapped = true
		}
		i = 0
	}
	for ; i < bucketCnt; i++ {
		offi := (i + it.offset) & (bucketCnt - 1)
		if isEmpty(b.tophash[offi]) || b.tophash[offi] == evacuatedEmpty {
			// TODO: emptyRest is hard to use here, as we start iterating
			// in the middle of a bucket. It's feasible, just tricky.
			continue
		}
		k := b.keys[offi]
		if checkBucket != noCheck && !m.sameSizeGrow() {
			// Special case: iterator was started during a grow to a larger size
			// and the grow is not done yet. We're working on a bucket whose
			// oldbucket has not been evacuated yet. Or at least, it wasn't
			// evacuated when we started the bucket. So we're iterating
			// through the oldbucket, skipping any keys that will go
			// to the other new bucket (each oldbucket expands to two
			// buckets during a grow).
			// If the item in the oldbucket is not destined for
			// the current new bucket in the iteration, skip it.
			hash := m.hasher(k)
			if int(hash&m.bucketMask()) != checkBucket {
				continue
			}
		}
		if b.tophash[offi] != evacuatedX && b.tophash[offi] != evacuatedY {
			// This is the golden data, we can return it.
			it.key = k
			it.elem = b.elems[offi]
		} else {
			// The hash table has grown since the iterator was started.
			// The golden data for this key is now somewhere else.
			// Check the current hash table for the data.
			// This code handles the case where the key
			// has been deleted, updated, or deleted and reinserted.
			// NOTE: we need to regrab the key as it has potentially been
			// updated to an equal() but not identical key (e.g. +0.0 vs -0.0).
			rk, re := m.mapaccessK(k)
			if rk == nil {
				continue // key has been deleted
			}
			it.key = *rk
			it.elem = *re
		}
		it.bucket = bucket
		if it.bptr != b { // avoid unnecessary write barrier; see issue 14921
			it.bptr = b
		}
		it.i = i + 1
		it.checkBucket = checkBucket
		return true
	}
	b = b.overflow
	i = 0
	goto next
}

func (m *Map[K, E]) hashGrow() {
	// If we've hit the load factor, get bigger.
	// Otherwise, there are too many overflow buckets,
	// so keep the same number of buckets and "grow" laterally.
	newsize := len(m.buckets) * 2
	if !overLoadFactor(m.count+1, len(m.buckets)) {
		newsize = len(m.buckets)
		m.flags |= sameSizeGrow
	}
	oldbuckets := m.buckets
	newbuckets := makeBucketArray[K, E](newsize)

	flags := m.flags &^ (iterator | oldIterator)
	if m.flags&iterator != 0 {
		flags |= oldIterator
	}
	// commit the grow (atomic wrt gc)
	m.flags = flags
	m.oldbuckets = &oldbuckets
	m.buckets = newbuckets
	m.nevacuate = 0
	m.noverflow = 0

	// the actual copying of the hash table data is done incrementally
	// by growWork() and evacuate().
}

// overLoadFactor reports whether count items placed in 1<<B buckets is over loadFactor.
func overLoadFactor(count int, nbuckets int) bool {
	return count > bucketCnt && uint64(count) > loadFactorNum*(uint64(nbuckets)/loadFactorDen)
}

// tooManyOverflowBuckets reports whether noverflow buckets is too many for a map with 1<<B buckets.
// Note that most of these overflow buckets must be in sparse use;
// if use was dense, then we'd have already triggered regular map growth.
func tooManyOverflowBuckets(noverflow uint32, nbuckets int) bool {
	// If the threshold is too low, we do extraneous work.
	// If the threshold is too high, maps that grow and shrink can hold on to lots of unused memory.
	// "too many" means (approximately) as many overflow buckets as regular buckets.
	// See incrnoverflow for more details.
	// The compiler doesn't see here that B < 16; mask B to generate shorter shift code.
	return noverflow >= uint32(nbuckets)
}

// growing reports whether h is growing. The growth may be to the same size or bigger.
func (m *Map[K, E]) growing() bool {
	return m.oldbuckets != nil
}

// sameSizeGrow reports whether the current growth is to a map of the same size.
func (m *Map[K, E]) sameSizeGrow() bool {
	return m.flags&sameSizeGrow != 0
}

func (m *Map[K, E]) bucketMask() uint64 {
	return uint64(len(m.buckets) - 1)
}

// oldbucketmask provides a mask that can be applied to calculate n % noldbuckets().
func (m *Map[K, E]) oldbucketmask() uint64 {
	return uint64(len(*m.oldbuckets) - 1)
}

func (m *Map[K, E]) growWork(bucket uint64) {
	// make sure we evacuate the oldbucket corresponding
	// to the bucket we're about to use
	m.evacuate(bucket & m.oldbucketmask())

	// evacuate one more oldbucket to make progress on growing
	if m.growing() {
		m.evacuate(m.nevacuate)
	}
}

func (m *Map[K, E]) bucketEvacuated(bucket uint64) bool {
	return evacuated(&(*m.oldbuckets)[bucket])
}

// evacDst is an evacuation destination.
type evacDst[K, E any] struct {
	b *bucket[K, E] // current destination bucket
	i int           // key/elem index into b
}

func (m *Map[K, E]) evacuate(oldbucket uint64) {
	b := &(*m.oldbuckets)[oldbucket]
	newbit := uint64(len(*m.oldbuckets))
	if !evacuated(b) {
		// TODO: reuse overflow buckets instead of using new ones, if there
		// is no iterator using the old buckets.  (If !oldIterator.)

		// xy contains the x and y (low and high) evacuation destinations.
		var xy [2]evacDst[K, E]
		x := &xy[0]
		x.b = &m.buckets[oldbucket]

		if !m.sameSizeGrow() {
			// Only calculate y pointers if we're growing bigger.
			// Otherwise GC can see bad pointers.
			y := &xy[1]
			y.b = &m.buckets[oldbucket+newbit]
		}

		for ; b != nil; b = b.overflow {
			for i := 0; i < bucketCnt; i++ {
				top := b.tophash[i]
				if isEmpty(top) {
					b.tophash[i] = evacuatedEmpty
					continue
				}
				if top < minTopHash {
					panic("bad map state")
				}
				var useY uint8
				if !m.sameSizeGrow() {
					// Compute hash to make our evacuation decision (whether we need
					// to send this key/elem to bucket x or bucket y).
					hash := m.hasher(b.keys[i])
					if hash&newbit != 0 {
						useY = 1
					}
				}

				if evacuatedX+1 != evacuatedY || evacuatedX^1 != evacuatedY {
					panic("bad evacuatedN")
				}

				b.tophash[i] = evacuatedX + useY // evacuatedX + 1 == evacuatedY
				dst := &xy[useY]                 // evacuation destination

				if dst.i == bucketCnt {
					dst.b = m.newoverflow(dst.b)
					dst.i = 0
				}
				dst.b.tophash[dst.i&(bucketCnt-1)] = top // mask dst.i as an optimization, to avoid a bounds che
				dst.b.keys[dst.i&(bucketCnt-1)] = b.keys[i]
				dst.b.elems[dst.i&(bucketCnt-1)] = b.elems[i]
				dst.i++
			}
		}
		// Unlink the overflow buckets & clear key/elem to help GC.
		if m.flags&oldIterator == 0 {
			b := &(*m.oldbuckets)[oldbucket]
			// Preserve b.tophash because the evacuation
			// state is maintained there.
			b.keys = [bucketCnt]K{}
			b.elems = [bucketCnt]E{}
			b.overflow = nil
		}
	}

	if oldbucket == m.nevacuate {
		m.advanceEvacuationMark(newbit)
	}
}

func (m *Map[K, E]) advanceEvacuationMark(newbit uint64) {
	m.nevacuate++
	// Experiments suggest that 1024 is overkill by at least an order of magnitude.
	// Put it in there as a safeguard anyway, to ensure O(1) behavior.
	stop := m.nevacuate + 1024
	if stop > newbit {
		stop = newbit
	}
	for m.nevacuate != stop && m.bucketEvacuated(m.nevacuate) {
		m.nevacuate++
	}
	if m.nevacuate == newbit { // newbit == # of oldbuckets
		// Growing is all done. Free old main bucket array.
		m.oldbuckets = nil
		m.flags &^= sameSizeGrow
	}
}
