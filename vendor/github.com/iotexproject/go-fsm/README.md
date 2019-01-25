# go-fsm

It is a lightweight finite state machine (FSM) implementation in Golang. It has been inspired by [the FSM used in Apache
Hadoop YARN](https://github.com/apache/hadoop/blob/16b70619a24cdcf5d3b0fcf4b58ca77238ccbe6d/hadoop-yarn-project/hadoop-yarn/hadoop-yarn-common/src/main/java/org/apache/hadoop/yarn/state/StateMachine.java):

- Thread safe
- Multiple destination states
- Dynamic destination state determination


## Installation

```
go get github.com/zjshen14/go-fsm
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/zjshen14/go-fsm"
)

type evt struct {
	t fsm.EventType
}

func (e *evt) Type() fsm.EventType { return e.t }

func main() {
	fsm, _ := fsm.NewBuilder().
		AddInitialState("s1").
		AddStates("s2", "s3").
		AddTransition("s1", "e1", func(event fsm.Event) (fsm.State, error) { return "s2", nil }, []fsm.State{"s2"}).
		AddTransition("s2", "e2", func(event fsm.Event) (fsm.State, error) { return "s3", nil }, []fsm.State{"s3"}).
		AddTransition("s3", "e3", func(event fsm.Event) (fsm.State, error) { return "s1", nil }, []fsm.State{"s1"}).
		Build()

	fmt.Println(fsm.CurrentState())
	fsm.Handle(&evt{t: "e1"})
	fmt.Println(fsm.CurrentState())
	fsm.Handle(&evt{t: "e2"})
	fmt.Println(fsm.CurrentState())
	fsm.Handle(&evt{t: "e3"})
	fmt.Println(fsm.CurrentState())
}
```

## License

go-fsm is under the Apache 2.0 license. See the LICENSE file for details.