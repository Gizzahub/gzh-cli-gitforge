package runtask

import (
	"sort"
	"strings"
)

// selectOwnedActiveTask scopes implicit completion to the current owner.
// Foreign and terminal executions can never become an implicit task target.
func selectOwnedActiveTask(states []DerivedTaskState, owner TaskOwner) (string, *TaskResponse) {
	var tasks []string
	for _, state := range states {
		delivery := state.Execution.Purpose == "" || state.Execution.Purpose == "delivery"
		if state.Status == TaskExecutionStatusActive && delivery && sameOwner(state.Execution.Owner, owner) {
			tasks = append(tasks, state.Execution.Task)
		}
	}
	sort.Strings(tasks)
	if len(tasks) == 1 {
		return tasks[0], nil
	}
	if len(tasks) == 0 {
		res := response(TaskExecutionStatusBlocked, "", "no active execution owned by "+owner.String()+" is eligible for bare run-finish", "gz-git run list")
		return "", &res
	}
	commands := make([]string, len(tasks))
	for i, task := range tasks {
		commands[i] = "gz-git run finish " + task
	}
	res := response(TaskExecutionStatusBlocked, "", "bare run-finish is ambiguous for "+owner.String()+": "+strings.Join(tasks, ", "), strings.Join(commands, "; "))
	return "", &res
}
