package mock_facebookgo_clock

//go:generate mockgen -package=mock_facebookgo_clock -destination=./mock_clock.go github.com/facebookgo/clock Clock
