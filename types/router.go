package types

// Router provides handlers for each transaction type.
type Router interface {
	AddRoute(r string, h Handler) Router
	Route(ctx Context, path string) Handler
}
