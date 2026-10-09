package arr

import (
	"context"
	"fmt"
)

// Command is one entry of the /command queue.
//
// Every app in the family runs its long work this way: the POST returns
// immediately with an id, and the work's outcome is only visible by reading
// the command back. A caller that does not poll has not learned whether the
// work happened — it has learned that the request was accepted.
type Command struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Status is the lifecycle: queued, started, completed, failed, aborted.
	Status string `json:"status"`
	// Result is the outcome of a COMPLETED command, and the trap in this
	// API: a command that linked nothing still completes, with result
	// "unsuccessful". Status alone never reports that.
	Result string `json:"result"`
	// Exception and Message carry the detail of a failure, and which of the
	// two is populated varies by command.
	Exception string `json:"exception"`
	Message   string `json:"message"`
}

// Command statuses and results, as the apps spell them.
const (
	CommandQueued       = "queued"
	CommandStarted      = "started"
	CommandCompleted    = "completed"
	CommandFailed       = "failed"
	CommandAborted      = "aborted"
	CommandSuccessful   = "successful"
	CommandUnsuccessful = "unsuccessful"
)

// Finished reports whether the command will not change again.
func (c Command) Finished() bool {
	switch c.Status {
	case CommandCompleted, CommandFailed, CommandAborted:
		return true
	default:
		return false
	}
}

// Detail returns whichever failure description the command carries.
func (c Command) Detail() string {
	switch {
	case c.Exception != "":
		return c.Exception
	case c.Message != "":
		return c.Message
	default:
		return "no detail given"
	}
}

// RunCommand queues one command and returns it as accepted.
//
// payload is the command body, whose shape is per-command; the only field
// every one of them has is "name".
func (c *Client) RunCommand(ctx context.Context, payload any) (Command, error) {
	var out Command
	if err := c.http.Do(ctx, "POST", c.path("command"), payload, &out); err != nil {
		return Command{}, fmt.Errorf("arr: queue command: %w", err)
	}
	return out, nil
}

// CommandStatus reads one queued command back.
func (c *Client) CommandStatus(ctx context.Context, id int64) (Command, error) {
	var out Command
	path := fmt.Sprintf("%s/%d", c.path("command"), id)
	if err := c.http.Do(ctx, "GET", path, nil, &out); err != nil {
		return Command{}, fmt.Errorf("arr: read command %d: %w", id, err)
	}
	return out, nil
}
