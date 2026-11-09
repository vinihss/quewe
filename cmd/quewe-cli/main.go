// Command quewe-cli is a terminal client for the quewe API.
//
// It reuses the same HTTP endpoints and DTOs as the server, avoiding
// duplication. Clean architecture: cmd → client.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/vinihss/quewe/services/quewe/internal/client"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	baseURL := envOrDefault("TASK_SERVICE_URL", envOrDefault("QUEWE_URL", "http://localhost:8080"))
	principal := envOrDefault("TASK_SERVICE_PRINCIPAL", envOrDefault("QUEWE_PRINCIPAL", "owner-1"))

	c := client.New(baseURL, principal)

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "health":
		if err := c.Health(); err != nil {
			fail(err)
		}
		fmt.Println("ok")

	case "tasks":
		handleTasks(c, args)

	case "task":
		if len(args) < 1 {
			fail(fmt.Errorf("usage: quewe-cli task <id>"))
		}
		out, err := c.GetTask(args[0])
		if err != nil {
			fail(err)
		}
		printJSON(out)

	case "workers":
		handleWorkers(c, args)

	case "executions":
		handleExecutions(c, args)

	case "execution":
		if len(args) < 1 {
			fail(fmt.Errorf("usage: quewe-cli execution <id>"))
		}
		out, err := c.GetExecution(args[0])
		if err != nil {
			fail(err)
		}
		printJSON(out)

	case "groups":
		handleGroups(c, args)

	case "group":
		if len(args) < 1 {
			fail(fmt.Errorf("usage: quewe-cli group <id>"))
		}
		out, err := c.GetGroup(args[0])
		if err != nil {
			fail(err)
		}
		printJSON(out)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func handleTasks(c *client.Client, args []string) {
	if len(args) > 0 && args[0] == "create" {
		if len(args) < 4 {
			fail(fmt.Errorf("usage: quewe-cli tasks create <name> <type> <capability> [run_at]"))
		}
		runAt := ""
		if len(args) >= 5 {
			runAt = args[4]
		}
		out, err := c.CreateTask(args[1], args[2], args[3], "once", runAt)
		if err != nil {
			fail(err)
		}
		printJSON(out)
		return
	}
	out, err := c.ListTasks(50, "")
	if err != nil {
		fail(err)
	}
	printJSON(out)
}

func handleWorkers(c *client.Client, args []string) {
	if len(args) > 0 && args[0] == "register" {
		if len(args) < 5 {
			fail(fmt.Errorf("usage: quewe-cli workers register <hostname> <version> <concurrency> <cap1,cap2,...>"))
		}
		var caps []string
		for _, cap := range strings.Split(args[4], ",") {
			if c := strings.TrimSpace(cap); c != "" {
				caps = append(caps, c)
			}
		}
		conc := 1
		fmt.Sscanf(args[3], "%d", &conc)
		out, err := c.RegisterWorker(args[1], args[2], conc, caps)
		if err != nil {
			fail(err)
		}
		printJSON(out)
		return
	}
	if len(args) > 0 && args[0] == "drain" {
		if len(args) < 2 {
			fail(fmt.Errorf("usage: quewe-cli workers drain <id>"))
		}
		if err := c.DrainWorker(args[1]); err != nil {
			fail(err)
		}
		fmt.Println("drained")
		return
	}
	out, err := c.ListWorkers()
	if err != nil {
		fail(err)
	}
	printJSON(out)
}

func handleExecutions(c *client.Client, args []string) {
	out, err := c.ListExecutions()
	if err != nil {
		fail(err)
	}
	printJSON(out)
}

func handleGroups(c *client.Client, args []string) {
	if len(args) > 0 && args[0] == "create" {
		if len(args) < 3 {
			fail(fmt.Errorf("usage: quewe-cli groups create <parent_exec_id> <policy> [member1,member2,...]"))
		}
		var members []string
		if len(args) >= 4 {
			for _, m := range strings.Split(args[3], ",") {
				if t := strings.TrimSpace(m); t != "" {
					members = append(members, t)
				}
			}
		}
		out, err := c.CreateGroup(args[1], args[2], members)
		if err != nil {
			fail(err)
		}
		printJSON(out)
		return
	}
	fail(fmt.Errorf("usage: quewe-cli groups create <parent_exec_id> <policy> [members]"))
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `quewe-cli — terminal client for quewe

Usage:
  quewe-cli health
  quewe-cli tasks [create <name> <type> <capability> [run_at]]
  quewe-cli task <id>
  quewe-cli workers [register <host> <version> <concurrency> <caps> | drain <id>]
  quewe-cli executions
  quewe-cli execution <id>
  quewe-cli groups create <parent_exec_id> <policy> [members]
  quewe-cli group <id>

Environment:
  QUEWE_URL (or TASK_SERVICE_URL)             base URL (default: http://localhost:8080)
  QUEWE_PRINCIPAL (or TASK_SERVICE_PRINCIPAL) principal ID (default: owner-1)`)
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}