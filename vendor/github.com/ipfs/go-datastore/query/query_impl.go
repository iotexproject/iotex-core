package query

import "sort"

func DerivedResults(qr Results, ch <-chan Result) Results {
	return &results{
		query: qr.Query(),
		proc:  qr.Process(),
		res:   ch,
	}
}

// NaiveFilter applies a filter to the results.
func NaiveFilter(qr Results, filter Filter) Results {
	ch := make(chan Result)
	go func() {
		defer close(ch)
		defer qr.Close()

		for e := range qr.Next() {
			if e.Error != nil || filter.Filter(e.Entry) {
				ch <- e
			}
		}
	}()

	return DerivedResults(qr, ch)
}

// NaiveLimit truncates the results to a given int limit
func NaiveLimit(qr Results, limit int) Results {
	ch := make(chan Result)
	go func() {
		defer close(ch)
		defer qr.Close()

		l := 0
		for e := range qr.Next() {
			if e.Error != nil {
				ch <- e
				continue
			}
			ch <- e
			l++
			if limit > 0 && l >= limit {
				break
			}
		}
	}()

	return DerivedResults(qr, ch)
}

// NaiveOffset skips a given number of results
func NaiveOffset(qr Results, offset int) Results {
	ch := make(chan Result)
	go func() {
		defer close(ch)
		defer qr.Close()

		sent := 0
		for e := range qr.Next() {
			if e.Error != nil {
				ch <- e
			}

			if sent < offset {
				sent++
				continue
			}
			ch <- e
		}
	}()

	return DerivedResults(qr, ch)
}

// NaiveOrder reorders results according to given orders.
// WARNING: this is the only non-stream friendly operation!
func NaiveOrder(qr Results, orders ...Order) Results {
	// Short circuit.
	if len(orders) == 0 {
		return qr
	}

	ch := make(chan Result)
	var entries []Entry
	go func() {
		defer close(ch)
		defer qr.Close()

		for e := range qr.Next() {
			if e.Error != nil {
				ch <- e
			}

			entries = append(entries, e.Entry)
		}
		sort.Slice(entries, func(i int, j int) bool {
			a, b := entries[i], entries[j]

			for _, cmp := range orders {
				switch cmp.Compare(a, b) {
				case 0:
				case -1:
					return true
				case 1:
					return false
				}
			}

			// This gives us a *stable* sort for free. We don't care
			// preserving the order from the underlying datastore
			// because it's undefined.
			return a.Key < b.Key
		})

		for _, e := range entries {
			ch <- Result{Entry: e}
		}
	}()

	return DerivedResults(qr, ch)
}

func NaiveQueryApply(q Query, qr Results) Results {
	if q.Prefix != "" {
		qr = NaiveFilter(qr, FilterKeyPrefix{q.Prefix})
	}
	for _, f := range q.Filters {
		qr = NaiveFilter(qr, f)
	}
	if len(q.Orders) > 0 {
		qr = NaiveOrder(qr, q.Orders...)
	}
	if q.Offset != 0 {
		qr = NaiveOffset(qr, q.Offset)
	}
	if q.Limit != 0 {
		qr = NaiveLimit(qr, q.Offset)
	}
	return qr
}

func ResultEntriesFrom(keys []string, vals [][]byte) []Entry {
	re := make([]Entry, len(keys))
	for i, k := range keys {
		re[i] = Entry{Key: k, Value: vals[i]}
	}
	return re
}
