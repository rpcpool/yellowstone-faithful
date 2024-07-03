package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestFirstSuccess(t *testing.T) {
	{
		jobGroup := NewJobGroup[int]()
		jobGroup.Add(func(context.Context) (int, error) {
			return 1, nil
		})

		val, err := jobGroup.Run(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if val != 1 {
			t.Errorf("expected 1, got %v", val)
		}
	}
	{
		jobGroup := NewJobGroup[int]()
		jobGroup.Add(func(context.Context) (int, error) {
			time.Sleep(1 * time.Second)
			return 1, nil
		})
		jobGroup.Add(func(context.Context) (int, error) {
			return 2, nil
		})

		val, err := jobGroup.Run(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if val != 2 {
			t.Errorf("expected 2, got %v", val)
		}
	}
	{
		jobGroup := NewJobGroup[int]()
		jobGroup.Add(func(context.Context) (int, error) {
			time.Sleep(1 * time.Second)
			return 1, nil
		})
		jobGroup.Add(func(context.Context) (int, error) {
			time.Sleep(2 * time.Second)
			return 2, nil
		})
		jobGroup.Add(func(context.Context) (int, error) {
			return 3, nil
		})

		startedAt := time.Now()
		val, err := jobGroup.Run(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if val != 3 {
			t.Errorf("expected 3, got %v", val)
		}
		if time.Since(startedAt) > time.Second {
			t.Errorf("expected less than 1 second, got %v", time.Since(startedAt))
		}
	}
	{
		// all functions fail
		jobGroup := NewJobGroup[int]()
		jobGroup.Add(func(context.Context) (int, error) {
			return -1, errors.New("error 1")
		})
		jobGroup.Add(func(context.Context) (int, error) {
			return -1, errors.New("error 2")
		})
		jobGroup.Add(func(context.Context) (int, error) {
			return -1, errors.New("error 3")
		})

		val, err := jobGroup.Run(context.Background())
		if err == nil {
			t.Errorf("expected error, got nil")
		}
		if val != 0 {
			t.Errorf("expected 0, got %v", val)
		}
		if !IsErrorSlice(err) {
			t.Errorf("expected error slice, got %v", err)
		}
		slice := err.(ErrorSlice)
		if len(slice) != 3 {
			t.Errorf("expected 3 errors, got %v", len(slice))
		}
		{
			// Cannot know the order of errors
			if slice.Filter(func(err error) bool {
				return err.Error() == "error 1"
			}).Error() != "ErrorSlice{\"error 1\"}" {
				t.Errorf("unexpected filtered error message")
			}
			if slice.Filter(func(err error) bool {
				return err.Error() == "error 2"
			}).Error() != "ErrorSlice{\"error 2\"}" {
				t.Errorf("unexpected filtered error message")
			}
			if slice.Filter(func(err error) bool {
				return err.Error() == "error 3"
			}).Error() != "ErrorSlice{\"error 3\"}" {
				t.Errorf("unexpected filtered error message")
			}
		}
	}
	{
		// some functions fail
		jobGroup := NewJobGroup[int]()
		jobGroup.Add(func(context.Context) (int, error) {
			return -1, errors.New("error 1")
		})
		jobGroup.Add(func(context.Context) (int, error) {
			return 2, nil
		})
		jobGroup.Add(func(context.Context) (int, error) {
			return -1, errors.New("error 3")
		})

		val, err := jobGroup.Run(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if val != 2 {
			t.Errorf("expected 2, got %v", val)
		}
	}
	{
		// two fail, two succeed at different times
		jobGroup := NewJobGroup[int]()
		jobGroup.Add(func(context.Context) (int, error) {
			time.Sleep(time.Millisecond * 100)
			return -1, errors.New("error 1")
		})
		jobGroup.Add(func(context.Context) (int, error) {
			time.Sleep(time.Millisecond * 200)
			return -1, errors.New("error 2")
		})
		jobGroup.Add(func(context.Context) (int, error) {
			time.Sleep(time.Millisecond * 300)
			return 3, nil
		})
		jobGroup.Add(func(context.Context) (int, error) {
			time.Sleep(time.Millisecond * 400)
			return 4, nil
		})

		startedAt := time.Now()
		val, err := jobGroup.Run(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if val != 3 {
			t.Errorf("expected 3, got %v", val)
		}
		if time.Since(startedAt) > time.Millisecond*350 {
			t.Errorf("expected less than 300ms, got %v", time.Since(startedAt))
		}
	}
	{
		// make sure that even if there is a success, the other functions still get executed
		jobGroup := NewJobGroup[int]()
		numCalled := new(atomic.Uint64)
		jobGroup.Add(func(context.Context) (int, error) {
			numCalled.Add(1)
			return 1, nil
		})
		jobGroup.Add(func(context.Context) (int, error) {
			numCalled.Add(1)
			return -1, errors.New("error 2")
		})
		jobGroup.Add(func(context.Context) (int, error) {
			numCalled.Add(1)
			time.Sleep(time.Millisecond * 100)
			return 123, nil
		})
		jobGroup.Add(func(context.Context) (int, error) {
			numCalled.Add(1)
			time.Sleep(time.Second)
			return 123, nil
		})

		startedAt := time.Now()
		val, err := jobGroup.Run(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if val != 1 {
			t.Errorf("expected 1, got %v", val)
		}
		if time.Since(startedAt) > time.Millisecond*100 {
			t.Errorf("expected less than 100ms, got %v", time.Since(startedAt))
		}
		time.Sleep(time.Second * 2)
		if numCalled.Load() != 4 {
			t.Errorf("expected 4, got %v", numCalled.Load())
		}
	}
	{
		// try with base valie of int
		jobGroup := NewJobGroup[int]()
		jobGroup.Add(func(context.Context) (int, error) {
			return 0, nil
		})
		jobGroup.Add(func(context.Context) (int, error) {
			time.Sleep(1 * time.Second)
			return 33, nil
		})

		val, err := jobGroup.Run(context.Background())
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if val != 0 {
			t.Errorf("expected 0, got %v", val)
		}
	}
}
