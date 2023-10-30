package metrics

type RegistrationRequest struct {
	MetricsName   string              `json:"metrics_name"`
	Labels        []map[string]string `json:"labels"`
	Prefix        string              `json:"prefix"`
	Type          string              `json:"type"`
	CheckSchedule string              `json:"check_schedule"`
	Help          string              `json:"help"`
}

type StoreRequest struct {
	MetricsName string              `json:"metrics_name"`
	Prefix      string              `json:"prefix"`
	Labels      []map[string]string `json:"labels"`
	Value       float64             `json:"value"`
}
