package main

import "fmt"

var codexHookEvents = []string{"SessionStart", "SessionEnd", "PreToolUse", "PostToolUse", "UserPromptSubmit", "Stop", "PermissionRequest", "Interrupt"}

func reconcileCodexHooks(data []byte, lgrassdPath string) ([]byte, bool, error) {
	registrations := make([]hookRegistration, 0, len(codexHookEvents))
	for _, event := range codexHookEvents {
		handler := hookHandler{
			Type:    "command",
			Command: fmt.Sprintf("LGRASS_HOOK_VENDOR=codex %s hook %s", lgrassdPath, event),
		}
		matcher := ""
		switch event {
		case "SessionStart":
			matcher = "startup|resume|clear|compact"
			handler.AdditionalContextLimit = 5000
		case "SessionEnd", "Interrupt":
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
