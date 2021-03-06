package executor

import "github.com/cloudfoundry-incubator/runtime-schema/models"

func EnvironmentVariablesToModel(envVars []EnvironmentVariable) []models.EnvironmentVariable {
	out := make([]models.EnvironmentVariable, len(envVars))
	for i, val := range envVars {
		out[i].Name = val.Name
		out[i].Value = val.Value
	}
	return out
}

func EnvironmentVariablesFromModel(envVars []models.EnvironmentVariable) []EnvironmentVariable {
	out := make([]EnvironmentVariable, len(envVars))
	for i, val := range envVars {
		out[i].Name = val.Name
		out[i].Value = val.Value
	}
	return out
}
