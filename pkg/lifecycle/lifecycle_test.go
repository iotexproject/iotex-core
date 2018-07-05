package lifecycle

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/test/mock/mock_lifecycle"
)

func TestLifecycle(t *testing.T) {
	mctrl := gomock.NewController(t)
	defer mctrl.Finish()

	ctx := context.Background()
	m := mock_lifecycle.NewMockStartStopper(mctrl)
	m.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
	m.EXPECT().Stop(gomock.Any()).Return(nil).Times(1)

	var lc Lifecycle
	lc.Add(m)
	assert.Nil(t, lc.OnStart(ctx))
	assert.Nil(t, lc.OnStop(ctx))
}

func TestLifecycleWithError(t *testing.T) {
	mctrl := gomock.NewController(t)
	defer mctrl.Finish()

	ctx := context.Background()
	m1 := mock_lifecycle.NewMockStartStopper(mctrl)
	m1.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
	m1.EXPECT().Stop(gomock.Any()).Return(nil).Times(1)

	err := errors.New("error")
	m2 := mock_lifecycle.NewMockStartStopper(mctrl)
	m2.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
	m2.EXPECT().Stop(gomock.Any()).Return(err).Times(1)

	var lc Lifecycle
	lc.AddModels(m1, m2)
	assert.Nil(t, lc.OnStart(ctx))
	assert.EqualError(t, lc.OnStop(ctx), err.Error())
}
