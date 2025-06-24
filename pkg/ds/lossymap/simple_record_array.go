package lossymap

type simpleRecordArray[TKey comparable, TValue any] struct {
	records         []Record[TKey, TValue]
	valueComparator ValueComparator[TValue]
}

// NewSimpleRecordArray creates a RecordArray that is backed by a slice
// that resides in memory.
//
// In order to discard invalid or expired entries, it assumes it
// receives the lowest permitted value as expiration data. It uses a
// ValueComparator to determine whether the value of a record to be
// returned is at least as high as the lowest permitted value.
func NewSimpleRecordArray[TKey comparable, TValue any](
	count int,
	valueComparator ValueComparator[TValue],
) RecordArray[TKey, TValue, TValue] {
	return &simpleRecordArray[TKey, TValue]{
		records:         make([]Record[TKey, TValue], count),
		valueComparator: valueComparator,
	}
}

func (ra *simpleRecordArray[TKey, TValue]) Get(index uint64, lowestValidValue TValue) (Record[TKey, TValue], error) {
	record := ra.records[index]
	if ra.valueComparator(&record.Value, &lowestValidValue) < 0 {
		return Record[TKey, TValue]{}, ErrRecordInvalidOrExpired
	}
	return record, nil
}

func (ra *simpleRecordArray[TKey, TValue]) Put(index uint64, record Record[TKey, TValue], lowestValidValue TValue) error {
	ra.records[index] = record
	return nil
}
