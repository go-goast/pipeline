/*
The MIT License (MIT)

Copyright (c) 2015 James Garfield

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package pipeline // import "goast.net/x/pipeline"

import (
	"sync"
)

//Any Chan type matches can implement Pipeline
type X interface{}
type Pipeline <-chan X

//Related type is generated for you
type _Fan []Pipeline

//Collect a given number of values off of a pipeline into a slice
func (pip Pipeline) Collect(done <-chan bool, num int) (result []X) {
	for i := 0; i < num; i++ {
		select {
		case val := <-pip:
			result = append(result, val)
		case <-done:
			return
		}
	}
	return
}

//Fan pipeline out over a given number of workers and fan results back in on a singe channel
func (pip Pipeline) Fan(done <-chan bool, workers int, fn func(X) X) Pipeline {
	return pip.FanOut(done, workers, fn).FanIn(done)
}

//Fan pipeline out over a given number of worker pipelines and return that Fan
func (pip Pipeline) FanOut(done <-chan bool, workers int, fn func(X) X) _Fan {
	fan := _Fan{}
	for i := 0; i < workers; i++ {
		fan = append(fan, pip.worker(done, fn))
	}

	return fan
}

//Add a filter to a pipeline to only allow values that return true for the provided function
func (pip Pipeline) Filter(done <-chan bool, fn func(X) bool) Pipeline {
	out := make(chan X)
	go func() {
		defer close(out)
		for val := range pip {
			if fn(val) {
				select {
				case out <- val:
				case <-done:
					return
				}
			}
		}
	}()
	return out
}

//Add a process onto the pipeline
func (pip Pipeline) Pipe(done <-chan bool, fn func(X) X) Pipeline {
	out := make(chan X)

	go func() {
		defer close(out)
		for val := range pip {
			select {
			case out <- fn(val):
			case <-done:
				return
			}
		}
	}()

	return out
}

//Create a worker pipeline on the current one
func (pip Pipeline) worker(done <-chan bool, fn func(X) X) Pipeline {
	out := make(chan X)
	go func() {
		defer close(out)
		for val := range pip {
			select {
			case out <- fn(val):
			case <-done:
				return
			}
		}
	}()
	return out
}

//Fan in the output of the pipelines in this Fan into a single channel.
func (fan _Fan) FanIn(done <-chan bool) Pipeline {
	var wg sync.WaitGroup
	out := make(chan X)

	output := func(pl Pipeline) {
		defer wg.Done()
		for val := range pl {
			select {
			case out <- val:
			case <-done:
				return
			}
		}
	}

	wg.Add(len(fan))
	for _, val := range fan {
		go output(val)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

//Apply a filter to each of the worker pipelines in this Fan
func (fan _Fan) Filter(done <-chan bool, fn func(X) bool) (result _Fan) {
	for _, pipe := range fan {
		result = append(result, pipe.Filter(done, fn))
	}
	return
}

//Add a process onto each worker in this Fan
func (fan _Fan) Pipe(done <-chan bool, fn func(X) X) (result _Fan) {
	for _, pipe := range fan {
		result = append(result, pipe.Pipe(done, fn))
	}
	return
}
