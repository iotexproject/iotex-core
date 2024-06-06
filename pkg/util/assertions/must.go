package assertions

func MustNoErrorV[V any](v V, err error) V {
	if err != nil {
		panic(err)
	}
	return v
}

func MustBeTrueV[V any](v V, ok bool) V {
	if !ok {
		panic("must ok")
	}
	return v
}
