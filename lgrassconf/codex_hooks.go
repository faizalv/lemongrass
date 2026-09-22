package main

import "fmt"

var codexHookEvents = []string{"SessionStart", "SessionEnd", "PreToolUse", "PostToolUse"}

func reconcileCodexHooks(data []byte, lgrassPath string) ([]byte, bool, error) {
	registrations := make([]hookRegistration, 0, len(codexHookEvents))
	for _, event := range codexHookEvents {
		handler := hookHandler{
			Type:    "command",
			Command: fmt.Sprintf("LGRASS_HOOK_VENDOR=codex %s hook %s", lgrassPath, event),
		}
		matcher := ""
		switch event {
		case "SessionStart":
			matcher = "startup|resume|clear|compact"
			handler.AdditionalContextLimit = 5000
		case "SessionEnd":
			handler.Timeout = 3
		}
		registrations = append(registrations, hookRegistration{
			Event:   event,
			Matcher: matcher,
			Handler: handler,
		})
	}
	return reconcileHookGroups(data, registrations)
}
