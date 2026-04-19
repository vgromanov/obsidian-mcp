package obsidian

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePeriodicPeriod(t *testing.T) {
	t.Parallel()
	_, err := ParsePeriodicPeriod("hourly")
	require.Error(t, err)

	p, err := ParsePeriodicPeriod(" DAILY ")
	require.NoError(t, err)
	require.Equal(t, PeriodDaily, p)

	p2, err := ParsePeriodicPeriod("weekly")
	require.NoError(t, err)
	require.Equal(t, PeriodWeekly, p2)
}
