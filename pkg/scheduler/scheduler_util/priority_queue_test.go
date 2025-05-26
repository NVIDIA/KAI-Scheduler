// Copyright 2025 NVIDIA CORPORATION
// SPDX-License-Identifier: Apache-2.0

package scheduler_util

import (
	"reflect"
	"testing"

	"github.com/NVIDIA/KAI-scheduler/pkg/scheduler/api/common_info"
)

func TestPriorityQueue_PushAndPop(t *testing.T) {
	type fields struct {
		lessFn       common_info.LessFn
		maxQueueSize int
	}
	type args struct {
		previouslyPushed []interface{}
		it               interface{}
	}
	type want struct {
		mostPrioritized interface{}
		queueLength     int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   want
	}{
		{
			name: "add item",
			fields: fields{
				lessFn: func(l interface{}, r interface{}) bool {
					lv := l.(int)
					rv := r.(int)
					return lv < rv
				},
				maxQueueSize: QueueCapacityInfinite,
			},
			args: args{
				previouslyPushed: []interface{}{2, 3},
				it:               interface{}(1),
			},
			want: want{
				mostPrioritized: 1,
				queueLength:     3,
			},
		},
		{
			name: "add less prioritized item",
			fields: fields{
				lessFn: func(l interface{}, r interface{}) bool {
					lv := l.(int)
					rv := r.(int)
					return lv < rv
				},
				maxQueueSize: QueueCapacityInfinite,
			},
			args: args{
				previouslyPushed: []interface{}{1, 3},
				it:               interface{}(2),
			},
			want: want{
				mostPrioritized: 1,
				queueLength:     3,
			},
		},
		{
			name: "add item - limited queue size",
			fields: fields{
				lessFn: func(l interface{}, r interface{}) bool {
					lv := l.(int)
					rv := r.(int)
					return lv < rv
				},
				maxQueueSize: 2,
			},
			args: args{
				previouslyPushed: []interface{}{2, 3, 4},
				it:               interface{}(1),
			},
			want: want{
				mostPrioritized: 1,
				queueLength:     2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewPriorityQueue(tt.fields.lessFn, tt.fields.maxQueueSize)
			for _, queueFill := range tt.args.previouslyPushed {
				q.Push(queueFill)
			}
			q.Push(tt.args.it)

			if q.Len() != tt.want.queueLength {
				t.Errorf("got %v, want %v", q.Len(), tt.want.queueLength)
			}

			mostPrioritized := q.Pop()
			if !reflect.DeepEqual(mostPrioritized, tt.want.mostPrioritized) {
				t.Errorf("got %v, want %v", mostPrioritized, tt.want.mostPrioritized)
			}
		})
	}
}

func TestPriorityQueue_Peek(t *testing.T) {
	type fields struct {
		lessFn       common_info.LessFn
		maxQueueSize int
	}
	type args struct {
		previouslyPushed []interface{}
	}
	type want struct {
		mostPrioritized interface{}
		queueLength     int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   want
	}{
		{
			name: "basic peek",
			fields: fields{
				lessFn: func(l interface{}, r interface{}) bool {
					lv := l.(int)
					rv := r.(int)
					return lv < rv
				},
				maxQueueSize: QueueCapacityInfinite,
			},
			args: args{
				previouslyPushed: []interface{}{2, 3},
			},
			want: want{
				mostPrioritized: 2,
				queueLength:     2,
			},
		},
		{
			name: "no items",
			fields: fields{
				lessFn: func(l interface{}, r interface{}) bool {
					lv := l.(int)
					rv := r.(int)
					return lv < rv
				},
				maxQueueSize: QueueCapacityInfinite,
			},
			args: args{
				previouslyPushed: []interface{}{},
			},
			want: want{
				mostPrioritized: nil,
				queueLength:     0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := NewPriorityQueue(tt.fields.lessFn, tt.fields.maxQueueSize)
			for _, queueFill := range tt.args.previouslyPushed {
				q.Push(queueFill)
			}
			if q.Len() != tt.want.queueLength {
				t.Errorf("before peek, queue length got %v, want %v", q.Len(), tt.want.queueLength)
			}

			mostPrioritized := q.Peek()
			if !reflect.DeepEqual(mostPrioritized, tt.want.mostPrioritized) {
				t.Errorf("got %v, want %v", mostPrioritized, tt.want.mostPrioritized)
			}

			if q.Len() != tt.want.queueLength {
				t.Errorf("after peek, queue length got %v, want %v", q.Len(), tt.want.queueLength)
			}
		})
	}
}
