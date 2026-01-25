package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/steveyegge/gastown/internal/jules"
)

var (
	resumeFlag = flag.String("resume", "", "Session ID to resume")
)

func main() {
	flag.Parse()

	// 1. Initialize Client
	client, err := jules.NewClientFromEnv()
	if err != nil {
		log.Fatalf("Error initializing Jules client: %v\nMake sure JULES_API_KEY is set.", err)
	}

	var sessionID string

	// 2. Resume or Create Session
	if *resumeFlag != "" {
		sessionID = *resumeFlag
		fmt.Printf("Resuming session: %s\n", sessionID)
		// Verify session exists
		_, err := client.GetSession(sessionID)
		if err != nil {
			log.Fatalf("Failed to resume session %s: %v", sessionID, err)
		}
	} else {
		// Initial prompt is required to create a session in Jules API.
		// For Gas Town integration, we might wait for the first user input.
		// However, typical agent startup in Gas Town (e.g. Claude) often starts and waits.
		// Let's print a banner and wait for input to create the session.
		fmt.Println("Jules Agent (gt-jules)")
		fmt.Println("Waiting for initial prompt to start session...")
	}

	scanner := bufio.NewScanner(os.Stdin)

	// If we haven't created a session yet, block for first prompt
	if sessionID == "" {
		if scanner.Scan() {
			prompt := scanner.Text()
			// Create session
			// Note: We need a source. For now, let's assume one is configured or discoverable?
			// The API requires sourceContext.
			// Ideally we'd list sources and pick one, or use a flag.
			// For simplicity in this proof-of-concept, let's list sources and pick the first one
			// if not specified via some other mechanism (which we haven't built yet).
			// Let's look for JULES_SOURCE env var.

			sourceName := os.Getenv("JULES_SOURCE")
			if sourceName == "" {
				// Try to list sources
				resp, err := client.ListSources(1, "")
				if err != nil || len(resp.Sources) == 0 {
					log.Fatalf("No JULES_SOURCE env var and failed to list sources (or none found): %v", err)
				}
				sourceName = resp.Sources[0].Name
				fmt.Printf("Auto-selected source: %s\n", sourceName)
			}

			req := &jules.CreateSessionRequest{
				Prompt: prompt,
				SourceContext: &jules.SourceContext{
					Source: sourceName,
					GithubRepoContext: &jules.GithubRepoContext{
						StartingBranch: "main", // Default
					},
				},
				AutomationMode: "AUTO_CREATE_PR",
			}

			sess, err := client.CreateSession(req)
			if err != nil {
				log.Fatalf("Failed to create session: %v", err)
			}
			sessionID = sess.Name // "sessions/..."
			// Extract ID part if needed, but API usually uses full name
			// If Name is "sessions/123", we use that.

			fmt.Printf("Session started: %s\n", sessionID)
			// Poll for response to this initial prompt
			pollActivities(client, sessionID)
		} else {
			return // EOF
		}
	}

	// 3. Main Loop
	fmt.Print("> ")
	for scanner.Scan() {
		text := scanner.Text()
		if strings.TrimSpace(text) == "" {
			continue
		}

		if err := client.SendMessage(sessionID, text); err != nil {
			log.Printf("Error sending message: %v", err)
			continue
		}

		// Poll for updates
		pollActivities(client, sessionID)
		fmt.Print("> ")
	}
}

// pollActivities checks for new activities and prints them.
// This is a naive implementation that tracks seen activity IDs.
func pollActivities(client *jules.Client, sessionID string) {
	seenActivities := make(map[string]bool)
	// In a real implementation, we'd persist seen state or use page tokens smarter.
	// For this REPL loop, we just want to show the response to the *current* action.
	// But Jules processes asynchronously.

	// We'll poll until we see a "stopped" state or a timeout,
	// OR (simpler for now) poll for a few seconds or until we see an agent response.

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	timeout := time.After(30 * time.Second) // Wait up to 30s for response

	for {
		select {
		case <-timeout:
			return
		case <-ticker.C:
			resp, err := client.ListActivities(sessionID, 100, "")
			if err != nil {
				log.Printf("Error polling activities: %v", err)
				continue
			}

			// Process activities reverse chronologically or check timestamps?
			// The API returns most recent first usually? Docs don't specify sort order clearly in snippet.
			// Let's assume standard list order.

			// We ideally want to print new stuff.
			// For this MVP, let's just print the latest Agent activity if we haven't seen it.
			// (A robust CLI would need better state tracking).

			// Let's iterate backwards if possible or just print what we find.
			for _, act := range resp.Activities {
				if seenActivities[act.ID] {
					continue
				}
				seenActivities[act.ID] = true

				if act.Originator == "agent" {
					if act.ProgressUpdated != nil {
						fmt.Printf("[Agent Progress] %s %s\n", act.ProgressUpdated.Title, act.ProgressUpdated.Description)
					}
					if len(act.Artifacts) > 0 {
						for _, art := range act.Artifacts {
							if art.BashOutput != nil {
								fmt.Printf("[Bash] %s\n%s\nExit: %d\n", art.BashOutput.Command, art.BashOutput.Output, art.BashOutput.ExitCode)
							}
							if art.ChangeSet != nil {
								fmt.Println("[ChangeSet] Changes generated")
							}
						}
					}
					// If plan generated
					if act.PlanGenerated != nil {
						fmt.Println("[Plan Generated]")
						for _, step := range act.PlanGenerated.Plan.Steps {
							fmt.Printf("  %d. %s\n", step.Index, step.Title)
						}
						// Auto-approve? CLI flag?
						// For now, assume auto-approved by default in CreateSession unless flag set.
					}
				}
			}
		}
	}
}
