package queue

import "encoding/json"

// BuildJobMethodConfig merges queue method_config with the job payload so the
// effective execution input is identical across direct and worker runtimes.
func BuildJobMethodConfig(methodConfigJSON, payloadJSON []byte) map[string]any {
	var methodConfig map[string]any
	if len(methodConfigJSON) > 0 {
		_ = json.Unmarshal(methodConfigJSON, &methodConfig)
	}
	if methodConfig == nil {
		methodConfig = make(map[string]any)
	}

	if len(payloadJSON) == 0 || string(payloadJSON) == "null" {
		return methodConfig
	}

	var payload any
	if err := json.Unmarshal(payloadJSON, &payload); err == nil {
		methodConfig["payload"] = payload
	}

	return methodConfig
}
