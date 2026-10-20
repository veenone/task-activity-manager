package jira

const (
	datasetParamText      = "TEXT"
	datasetParamList      = "LIST"
	datasetListScopeAdHoc = "PARAMETER_AD_HOC"
)

func normalizedDatasetParameterType(parameterType string) string {
	if parameterType == "" {
		return datasetParamText
	}
	return parameterType
}
