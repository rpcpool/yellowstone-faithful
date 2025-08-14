package continuity

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCont(t *testing.T) {
	{
		c := New()
		err := c.Thenf("step 0", func() error {
			return nil
		}).Err()
		require.NoError(t, err)
	}
	{
		c := New()
		err := c.Thenf("step 0", func() error {
			return nil
		}).
			Thenf("step 1", func() error {
				return nil
			}).
			Thenf("step 2", func() error {
				return nil
			}).Err()
		require.NoError(t, err)
	}
	{
		step0Executed := false
		step1Executed := false
		step2Executed := false
		step3Executed := false
		c := New()
		err := c.
			Thenf("step 0", func() error {
				step0Executed = true
				return nil
			}).
			Thenf("step 1", func() error {
				step1Executed = true
				return nil
			}).
			Thenf("step 2", func() error {
				step2Executed = true
				return errors.New("step 2 error")
			}).
			Thenf("step 3", func() error {
				step3Executed = true
				return nil
			}).
			Err()
		require.Error(t, err)
		require.Equal(t, "step 2 error", err.Error())

		require.True(t, step0Executed)
		require.True(t, step1Executed)
		require.True(t, step2Executed)
		require.False(t, step3Executed)
	}
	{
		step0Executed := false
		step1Executed := false
		step2Executed := false
		step3Executed := false
		c := New()
		err := c.
			Thenf("step 0", func() error {
				step0Executed = true
				return nil
			}).
			Thenf("step 1", func() error {
				step1Executed = true
				return nil
			}).
			Then("step 2",
				func() error {
					step2Executed = true
					return errors.New("step 2 error 1")
				}(),
				errors.New("step 2 error 2"),
			).
			Thenf("step 3", func() error {
				step3Executed = true
				return nil
			}).
			Err()
		require.Error(t, err)
		require.Equal(t, "multiple errors: step 2 error 1, step 2 error 2", err.Error())

		require.True(t, step0Executed)
		require.True(t, step1Executed)
		require.True(t, step2Executed)
		require.False(t, step3Executed)
	}
}
