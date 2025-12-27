// Package command provides builders for KASA device commands.
package command

// Command represents a KASA protocol command.
// Commands are nested JSON: {"namespace": {"action": {args}}}
type Command map[string]map[string]interface{}

// New creates a new command with the given namespace and action.
func New(namespace, action string, args map[string]interface{}) Command {
	if args == nil {
		args = make(map[string]interface{})
	}
	return Command{
		namespace: {
			action: args,
		},
	}
}

// Merge combines multiple commands into a single request.
// This allows sending multiple commands in one TCP transaction.
func Merge(commands ...Command) Command {
	result := make(Command)
	for _, cmd := range commands {
		for namespace, methods := range cmd {
			if result[namespace] == nil {
				result[namespace] = make(map[string]interface{})
			}
			for action, args := range methods {
				result[namespace][action] = args
			}
		}
	}
	return result
}

// Namespaces used by different device types.
const (
	// Plug namespaces
	NSSystem = "system"
	NSEmeter = "emeter"

	// Bulb namespaces
	NSLightingService = "smartlife.iot.smartbulb.lightingservice"
	NSBulbEmeter      = "smartlife.iot.common.emeter"

	// Light strip namespace
	NSLightStrip = "smartlife.iot.lightStrip"

	// Context for child devices (power strips)
	NSContext = "context"
)
