package protocol

import "context"

type (
	lazyViews struct {
		v      Views
		loader func() Views
	}
)

// NewLazyViews creates a lazy-loaded Views
func NewLazyViews(loader func() Views) Views {
	return &lazyViews{
		loader: loader,
	}
}

func (lv *lazyViews) ensureLoaded() {
	if lv.v == nil {
		lv.v = lv.loader()
	}
}

func (lv *lazyViews) Snapshot() int {
	if lv.v == nil {
		return 0
	}
	return lv.v.Snapshot()
}

func (lv *lazyViews) Revert(id int) error {
	if lv.v == nil {
		return nil
	}
	return lv.v.Revert(id)
}

func (lv *lazyViews) Fork() Views {
	lv.ensureLoaded()
	return lv.v.Fork()
}

func (lv *lazyViews) Commit(ctx context.Context, sm StateManager) error {
	lv.ensureLoaded()
	return lv.v.Commit(ctx, sm)
}

func (lv *lazyViews) Read(name string) (View, error) {
	lv.ensureLoaded()
	return lv.v.Read(name)
}

func (lv *lazyViews) Write(name string, v View) {
	lv.ensureLoaded()
	lv.v.Write(name, v)
}
